package handler

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"ai-sell/gateway/internal/store"
)

func TestIdempotencyKeyValidation(t *testing.T) {
	if !validIdempotencyKey("request-01") {
		t.Fatal("ordinary request key should be accepted")
	}
	if validIdempotencyKey("") || validIdempotencyKey("contains space") {
		t.Fatal("empty and whitespace request keys must be rejected")
	}
}

func TestIdempotencyRequestHashSeparatesIntent(t *testing.T) {
	if idempotencyRequestHash("package", "a") == idempotencyRequestHash("package", "b") {
		t.Fatal("different intents must not share a request hash")
	}
	if idempotencyRequestHash("package", "a") == idempotencyRequestHash("redeem", "a") {
		t.Fatal("scopes must be isolated")
	}
}

func TestIdempotencyReplayAndConflictDB(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	uid := seedUser(t, ctx, db)
	t.Cleanup(func() { deleteFixture(t, ctx, db, uid) })
	key := "idem-test-" + randCode(12)
	t.Cleanup(func() {
		_, _ = db.Exec(ctx, `DELETE FROM idempotency_request WHERE user_id=$1 AND request_key=$2`, uid, key)
	})

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if cached, err := claimIdempotency(ctx, tx, uid, "market_order", key, `{"cart":true}`); err != nil || cached != nil {
		t.Fatalf("first claim: cached=%s err=%v", cached, err)
	}
	want := map[string]any{"ok": true, "order_id": "fixture"}
	if err := completeIdempotency(ctx, tx, uid, "market_order", key, want); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	cached, err := claimIdempotency(ctx, tx, uid, "market_order", key, `{"cart":true}`)
	var got map[string]any
	decodeErr := json.Unmarshal(cached, &got)
	if err != nil || decodeErr != nil || got["ok"] != true || got["order_id"] != "fixture" {
		t.Fatalf("replay: cached=%s err=%v decode=%v", cached, err, decodeErr)
	}
	_ = tx.Rollback(ctx)

	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = claimIdempotency(ctx, tx, uid, "market_order", key, `{"cart":false}`)
	if !errors.Is(err, errIdempotencyMismatch) {
		t.Fatalf("different intent error=%v, want mismatch", err)
	}
	_ = tx.Rollback(ctx)
}
