package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	errIdempotencyMismatch = errors.New("idempotency key reused with a different request")
	errIdempotencyPending  = errors.New("idempotency request is still processing")
)

func validIdempotencyKey(key string) bool {
	if len(key) < 8 || len(key) > 128 {
		return false
	}
	for _, c := range key {
		if c <= ' ' || c > '~' {
			return false
		}
	}
	return true
}

func idempotencyRequestHash(scope, intent string) string {
	sum := sha256.Sum256([]byte(scope + "\x00" + intent))
	return hex.EncodeToString(sum[:])
}

func idempotencyKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if !validIdempotencyKey(key) {
		fail(w, http.StatusBadRequest, "Idempotency-Key wajib 8-128 karakter tanpa spasi")
		return "", false
	}
	return key, true
}

// claimIdempotency reserves one user intent in the same transaction as its effect.
// A duplicate waits for the first transaction, then replays its recorded response.
func claimIdempotency(ctx context.Context, tx pgx.Tx, uid, scope, key, intent string) (json.RawMessage, error) {
	hash := idempotencyRequestHash(scope, intent)
	tag, err := tx.Exec(ctx,
		`INSERT INTO idempotency_request (user_id, scope, request_key, request_hash)
		 VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`, uid, scope, key, hash)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 1 {
		return nil, nil
	}
	var existingHash string
	var response []byte
	if err := tx.QueryRow(ctx,
		`SELECT request_hash, response FROM idempotency_request
		 WHERE user_id=$1 AND scope=$2 AND request_key=$3`, uid, scope, key).
		Scan(&existingHash, &response); err != nil {
		return nil, err
	}
	if existingHash != hash {
		return nil, errIdempotencyMismatch
	}
	if len(response) == 0 {
		return nil, errIdempotencyPending
	}
	return json.RawMessage(response), nil
}

func completeIdempotency(ctx context.Context, tx pgx.Tx, uid, scope, key string, response any) error {
	body, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`UPDATE idempotency_request SET response=$1 WHERE user_id=$2 AND scope=$3 AND request_key=$4`,
		body, uid, scope, key)
	return err
}

func replayIdempotency(w http.ResponseWriter, body json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(append(body, '\n'))
}

func failIdempotency(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errIdempotencyMismatch):
		fail(w, http.StatusUnprocessableEntity, "Idempotency-Key tidak boleh dipakai untuk request berbeda")
	case errors.Is(err, errIdempotencyPending):
		fail(w, http.StatusConflict, "request dengan Idempotency-Key ini masih diproses")
	default:
		fail(w, http.StatusInternalServerError, "gagal memproses idempotency request")
	}
}
