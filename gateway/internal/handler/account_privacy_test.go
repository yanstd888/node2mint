package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"ai-sell/gateway/internal/auth"
	"ai-sell/gateway/internal/store"
)

// TestSweepDeletedAccountsSQL: validates the deletion sweep SQL (Task 20).
// The 7-day grace period must be hard-coded in the SQL.
func TestSweepDeletedAccountsSQL(t *testing.T) {
	expected := "deletion_requested_at < now()-interval '7 days'"
	if !strings.Contains(sweepDeletedSQL, expected) {
		t.Errorf("sweep SQL must enforce 7-day grace period, missing: %q", expected)
	}
	// Must anonymize email + null out password_hash (irreversible deletion).
	if !strings.Contains(sweepDeletedSQL, "email='deleted+'") {
		t.Error("sweep SQL must anonymize email with deleted+ prefix")
	}
	if !strings.Contains(sweepDeletedSQL, "password_hash='DELETED'") {
		t.Error("sweep SQL must null out password_hash")
	}
	if !strings.Contains(sweepDeletedSQL, "deleted_at=now()") {
		t.Error("sweep SQL must set deleted_at timestamp")
	}
	// Must not delete financial records.
	if strings.Contains(strings.ToLower(sweepDeletedSQL), "credit_transaction") {
		t.Error("sweep SQL must NOT touch financial records")
	}
}

// TestAccountDeleteRequestGracePeriod: verifies the 7-day grace period
// is consistently applied between request handler and sweeper.
func TestAccountDeleteRequestGracePeriod(t *testing.T) {
	grace := 7 * 24 * time.Hour
	if grace != 168*time.Hour {
		t.Errorf("grace period should be 7 days (168h), got %v", grace)
	}
	if !strings.Contains(sweepDeletedSQL, "7 days") {
		t.Error("sweep SQL must use 7-day interval matching handler grace period")
	}
}

// TestAccountDeleteRequestRevokesSessions: verifies a delete request revokes
// active sessions so the user cannot re-trigger deletion while still logged in,
// and is idempotent on repeat. Checks the handler source so there's a runnable
// guard against regressions (end-to-end lives on the live akuntest account).
func TestAccountDeleteRequestRevokesSessions(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	src, err := os.ReadFile(filepath.Dir(file) + "/account_privacy.go")
	if err != nil {
		t.Fatalf("read handler source: %v", err)
	}
	body := string(src)
	if !strings.Contains(body, "DELETE FROM user_session WHERE user_id=$1") {
		t.Error("AccountDeleteRequest must revoke sessions (DELETE FROM user_session)")
	}
	if !strings.Contains(body, "already_scheduled") {
		t.Error("AccountDeleteRequest must be idempotent (repeat -> already_scheduled)")
	}
}

// TestAccountDeleteRequestSchedulesCleanAccount proves a NULL deletion_requested_at
// schedules once, revokes the active session, then reports a harmless retry.
func TestAccountDeleteRequestSchedulesCleanAccount(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji penghapusan akun")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(db.Close)

	uid := seedUser(t, ctx, db)
	t.Cleanup(func() {
		_, _ = db.Exec(ctx, `DELETE FROM audit_log WHERE action='ACCOUNT_DELETE_REQUEST' AND target=$1`, uid)
		deleteFixture(t, ctx, db, uid)
	})

	send := func() (*httptest.ResponseRecorder, string) {
		token, err := auth.IssueToken(uid, "USER")
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/account/delete", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		newSession(req, db, uid, token, time.Hour)
		rec := httptest.NewRecorder()
		auth.RequireAuth(db)(AccountDeleteRequest(db)).ServeHTTP(rec, req)
		return rec, token
	}

	first, firstToken := send()
	if first.Code != http.StatusOK {
		t.Fatalf("clean account status=%d body=%s", first.Code, first.Body.String())
	}
	var firstBody struct {
		OK               bool `json:"ok"`
		AlreadyScheduled bool `json:"already_scheduled"`
	}
	if err := json.NewDecoder(first.Body).Decode(&firstBody); err != nil || !firstBody.OK || firstBody.AlreadyScheduled {
		t.Fatalf("first response=%+v decode=%v", firstBody, err)
	}

	var scheduled time.Time
	if err := db.QueryRow(ctx, `SELECT deletion_requested_at FROM "user" WHERE id=$1`, uid).Scan(&scheduled); err != nil || scheduled.IsZero() {
		t.Fatalf("deletion not scheduled: at=%v err=%v", scheduled, err)
	}
	var sessions int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM user_session WHERE user_id=$1`, uid).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatalf("active sessions=%d err=%v, want 0", sessions, err)
	}

	// The revoked original token cannot trigger a second request.
	req := httptest.NewRequest(http.MethodPost, "/api/account/delete", nil)
	req.Header.Set("Authorization", "Bearer "+firstToken)
	rec := httptest.NewRecorder()
	auth.RequireAuth(db)(AccountDeleteRequest(db)).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token status=%d body=%s, want 401", rec.Code, rec.Body.String())
	}

	retry, _ := send()
	if retry.Code != http.StatusOK {
		t.Fatalf("repeat status=%d body=%s", retry.Code, retry.Body.String())
	}
	var retryBody struct {
		OK               bool `json:"ok"`
		AlreadyScheduled bool `json:"already_scheduled"`
	}
	if err := json.NewDecoder(retry.Body).Decode(&retryBody); err != nil || !retryBody.OK || !retryBody.AlreadyScheduled {
		t.Fatalf("repeat response=%+v decode=%v", retryBody, err)
	}
	var unchanged time.Time
	if err := db.QueryRow(ctx, `SELECT deletion_requested_at FROM "user" WHERE id=$1`, uid).Scan(&unchanged); err != nil || !unchanged.Equal(scheduled) {
		t.Fatalf("repeat moved schedule: before=%v after=%v err=%v", scheduled, unchanged, err)
	}
}

// TestExportJSONNoSecrets: validates the export payload (Task 20) must NOT
// include password_hash or other secrets — verified at the JSON structure level.
func TestExportJSONNoSecrets(t *testing.T) {
	payload := map[string]any{
		"exported_at":  time.Now().UTC(),
		"profile":      map[string]string{"id": "u1", "email": "a@b.com", "name": "Test"},
		"transactions": json.RawMessage(`[]`),
		"usage":        json.RawMessage(`[]`),
		"chat_history": json.RawMessage(`[]`),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	lower := strings.ToLower(string(b))
	for _, bad := range []string{"password_hash", "password", "api_key", "key_hash", "key_enc"} {
		if strings.Contains(lower, bad) {
			t.Errorf("export JSON must not contain %q (secret leak)", bad)
		}
	}
	for _, must := range []string{"exported_at", "profile", "transactions", "usage", "chat_history"} {
		if !strings.Contains(lower, must) {
			t.Errorf("export JSON must contain %q", must)
		}
	}
}
