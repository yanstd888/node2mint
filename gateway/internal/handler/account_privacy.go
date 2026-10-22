package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func AccountExport(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var email, name string
		if err := db.QueryRow(r.Context(), `SELECT email,COALESCE(name,'') FROM "user" WHERE id=$1`, uid).Scan(&email, &name); err != nil {
			fail(w, http.StatusNotFound, "account not found")
			return
		}
		var transactions, usage, histories []byte
		_ = db.QueryRow(r.Context(), `SELECT COALESCE(json_agg(x),'[]'::json) FROM (SELECT id,amount,type,status,description,created_at FROM credit_transaction WHERE user_id=$1 ORDER BY created_at DESC) x`, uid).Scan(&transactions)
		_ = db.QueryRow(r.Context(), `SELECT COALESCE(json_agg(x),'[]'::json) FROM (SELECT model,input_tokens,output_tokens,cache_tokens,cost, status, created_at FROM usage_log WHERE user_id=$1 ORDER BY created_at DESC LIMIT 10000) x`, uid).Scan(&usage)
		_ = db.QueryRow(r.Context(), `SELECT COALESCE(json_agg(x),'[]'::json) FROM (SELECT id,title,model,messages,created_at,updated_at FROM chat_history WHERE user_id=$1 ORDER BY updated_at DESC) x`, uid).Scan(&histories)
		w.Header().Set("Content-Disposition", `attachment; filename="node2mint-data.json"`)
		ok(w, map[string]any{"exported_at": time.Now().UTC(), "profile": map[string]string{"id": uid, "email": email, "name": name}, "transactions": json.RawMessage(transactions), "usage": json.RawMessage(usage), "chat_history": json.RawMessage(histories)})
	}
}

func AccountDeleteRequest(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "could not schedule account deletion")
			return
		}
		defer tx.Rollback(r.Context())

		// Lock the account row first: NULL is valid on a clean account, and the
		// lock makes concurrent delete clicks choose exactly one first request.
		var already pgtype.Timestamptz
		err = tx.QueryRow(r.Context(), `SELECT deletion_requested_at FROM "user" WHERE id=$1 FOR UPDATE`, uid).Scan(&already)
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, http.StatusNotFound, "account not found")
			return
		}
		if err != nil {
			fail(w, http.StatusInternalServerError, "could not schedule account deletion")
			return
		}
		if already.Valid {
			ok(w, map[string]any{"ok": true, "already_scheduled": true, "grace_until": already.Time.Add(7 * 24 * time.Hour)})
			return
		}

		var requested time.Time
		if err := tx.QueryRow(r.Context(), `UPDATE "user" SET deletion_requested_at=now(), updated_at=now() WHERE id=$1 RETURNING deletion_requested_at`, uid).Scan(&requested); err != nil {
			fail(w, http.StatusInternalServerError, "could not schedule account deletion")
			return
		}
		if _, err := tx.Exec(r.Context(), `UPDATE api_key SET is_active=false, revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1`, uid); err != nil {
			fail(w, http.StatusInternalServerError, "could not revoke api keys")
			return
		}
		if _, err := tx.Exec(r.Context(), `DELETE FROM user_session WHERE user_id=$1`, uid); err != nil {
			fail(w, http.StatusInternalServerError, "could not revoke sessions")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "could not schedule account deletion")
			return
		}

		audit(r.Context(), db, uid, "ACCOUNT_DELETE_REQUEST", uid, nil, map[string]any{"grace_days": 7}, ipOf(r))
		ok(w, map[string]any{"ok": true, "grace_until": requested.Add(7 * 24 * time.Hour)})
	}
}

func StartAccountDeletionSweeper(db DB, every time.Duration) {
	go func() {
		for {
			SweepDeletedAccounts(context.Background(), db)
			time.Sleep(every)
		}
	}()
}

// sweepDeletedSQL is the SQL for the 7-day grace period account deletion sweeper.
// Extracted as a package-level const so tests can verify the grace period and
// anonymization logic without a live database (Task 20).
const sweepDeletedSQL = `WITH gone AS (SELECT id FROM "user" WHERE deletion_requested_at < now()-interval '7 days' AND deleted_at IS NULL)
		UPDATE "user" u SET email='deleted+'||u.id::text||'@invalid.local', name='[deleted]', password_hash='DELETED', deleted_at=now(), updated_at=now()
		FROM gone WHERE u.id=gone.id`

func SweepDeletedAccounts(ctx context.Context, db DB) {
	_, err := db.Exec(ctx, sweepDeletedSQL)
	if err != nil {
		return
	}
	_, _ = db.Exec(ctx, `DELETE FROM chat_history WHERE user_id IN (SELECT id FROM "user" WHERE deleted_at IS NOT NULL AND deleted_at > now()-interval '1 hour')`)
}
