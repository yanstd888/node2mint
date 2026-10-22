package handler

// Port new-api: kode redeem — user tukar kode jadi saldo; admin membuat,
// mendaftar, menghapus. Semua klaim saldo lewat satu transaksi: lock baris
// `redeem_code` FOR UPDATE, cek masa berlaku/kuota pakai, tambah saldo, catat
// credit_transaction type REDEEM, tandai terpakai.

import (
	"context"
	"crypto/rand"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func accessDaysAllowed(remaining float64) bool { return remaining > 0 }

func redeemRewardAllowed(kind string, amount float64) bool {
	return (kind == "WALLET" || kind == "API_TOKEN" || kind == "ACCESS_DAYS" || kind == "BOOST" || kind == "VERIFIED_30D") && amount > 0 &&
		(kind == "WALLET" || amount == math.Trunc(amount))
}

// grantRedeemTokens keeps redeem token credits separate from marketplace money.
func grantRedeemTokens(ctx context.Context, tx pgx.Tx, userID string, tokens int) error {
	var pkgID string
	err := tx.QueryRow(ctx, `SELECT id FROM token_package WHERE name='Reward Redeem' AND token_count=$1 LIMIT 1`, tokens).Scan(&pkgID)
	if err != nil {
		err = tx.QueryRow(ctx, `INSERT INTO token_package (name, token_count, price, duration_days, is_active) VALUES ('Reward Redeem',$1,0,30,false) ON CONFLICT (lower(name), token_count) DO UPDATE SET name=EXCLUDED.name RETURNING id`, tokens).Scan(&pkgID)
		if err != nil {
			return err
		}
	}
	return grantReferralTokens(ctx, tx, userID, pkgID, tokens, 30, "redeem")
}

// Redeem: POST /api/redeem {code} — atomik; kode yang sama tidak bisa dipakai
// dua kali oleh user berbeda pada saat bersamaan (row lock).
func Redeem(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		if !RequireTelegramFeatureHTTP(w, r, db, telegramFeatureRedeem) {
			return
		}
		var in struct {
			Code string `json:"code"`
		}
		if err := decode(r, &in); err != nil || strings.TrimSpace(in.Code) == "" {
			fail(w, http.StatusBadRequest, "code wajib")
			return
		}
		code := strings.ToUpper(strings.TrimSpace(in.Code))
		requestKey, valid := idempotencyKey(w, r)
		if !valid {
			return
		}

		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		defer tx.Rollback(r.Context())
		cached, err := claimIdempotency(r.Context(), tx, uid, "redeem", requestKey, code)
		if err != nil {
			failIdempotency(w, err)
			return
		}
		if cached != nil {
			replayIdempotency(w, cached)
			return
		}

		var id string
		var amount float64
		var rewardType string
		var maxUses *int
		var usedCount int
		var expires *time.Time
		var redeemedBy *string
		err = tx.QueryRow(r.Context(),
			`SELECT id, amount, reward_type, max_uses, used_count, expires_at, redeemed_by
			   FROM redeem_code WHERE code=$1 FOR UPDATE`, code).
			Scan(&id, &amount, &rewardType, &maxUses, &usedCount, &expires, &redeemedBy)
		if err != nil {
			fail(w, http.StatusNotFound, "kode tidak ditemukan")
			return
		}
		if expires != nil && expires.Before(time.Now()) {
			fail(w, http.StatusBadRequest, "kode kedaluwarsa")
			return
		}
		if !redeemRewardAllowed(rewardType, amount) {
			fail(w, http.StatusBadRequest, "reward kode tidak valid")
			return
		}
		if rewardType == "ACCESS_DAYS" {
			var remaining float64
			if err := tx.QueryRow(r.Context(), `SELECT COALESCE(SUM(remaining),0) FROM user_token_package WHERE user_id=$1`, uid).Scan(&remaining); err != nil {
				fail(w, http.StatusInternalServerError, "gagal memeriksa token")
				return
			}
			if !accessDaysAllowed(remaining) {
				fail(w, http.StatusBadRequest, "masa aktif hanya dapat ditambah bila token masih tersisa")
				return
			}
		}
		if maxUses == nil {
			// Sekali pakai: redeemed_by adalah penanda "sudah dipakai".
			if redeemedBy != nil {
				fail(w, http.StatusBadRequest, "kode sudah dipakai")
				return
			}
		} else if usedCount >= *maxUses {
			fail(w, http.StatusBadRequest, "kode sudah habis")
			return
		}
		// PK redeem_claim adalah pagar atomik satu user satu kali, termasuk kode
		// multi-pakai. Rollback bila reward gagal sehingga kode tidak hangus.
		claim, err := tx.Exec(r.Context(),
			`INSERT INTO redeem_claim (redeem_code_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, id, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memvalidasi klaim kode")
			return
		}
		if claim.RowsAffected() == 0 {
			fail(w, http.StatusConflict, "kode ini sudah Anda gunakan")
			return
		}

		out := map[string]any{"ok": true, "amount": amount, "reward_type": rewardType}
		switch rewardType {
		case "WALLET":
			var balance float64
			if err := tx.QueryRow(r.Context(), `UPDATE "user" SET wallet_balance = wallet_balance + $1 WHERE id=$2 RETURNING wallet_balance`, amount, uid).Scan(&balance); err != nil {
				fail(w, http.StatusInternalServerError, "gagal menambah saldo")
				return
			}
			if _, err := tx.Exec(r.Context(), `INSERT INTO credit_transaction (user_id, amount, type, status, description) VALUES ($1,$2,'REDEEM','COMPLETED',$3)`, uid, amount, "Redeem kode "+code); err != nil {
				fail(w, http.StatusInternalServerError, "gagal mencatat transaksi")
				return
			}
			out["balance"] = balance
		case "API_TOKEN":
			if err := grantRedeemTokens(r.Context(), tx, uid, int(amount)); err != nil {
				fail(w, http.StatusInternalServerError, "gagal menambah token API")
				return
			}
			out["tokens"] = int(amount)
		case "ACCESS_DAYS":
			if _, err := tx.Exec(r.Context(), `UPDATE user_token_package SET expires_at=GREATEST(expires_at, now()) + make_interval(days=>$1), effective_expires_at=GREATEST(expires_at, now()) + make_interval(days=>$1) WHERE user_id=$2 AND remaining>0`, int(amount), uid); err != nil {
				fail(w, http.StatusInternalServerError, "gagal memperpanjang masa aktif")
				return
			}
			out["days"] = int(amount)
		case "BOOST":
			var credits int
			if err := tx.QueryRow(r.Context(), `UPDATE market_store SET boost_credits=boost_credits+$1, updated_at=now() WHERE user_id=$2 RETURNING boost_credits`, int(amount), uid).Scan(&credits); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					fail(w, http.StatusBadRequest, "buka toko sebelum memakai kode Boost")
					return
				}
				fail(w, http.StatusInternalServerError, "gagal menambah Boost")
				return
			}
			out["boost_credits"] = credits
		case "VERIFIED_30D":
			var until time.Time
			if err := tx.QueryRow(r.Context(), `UPDATE market_store SET is_verified=true, verified_until=GREATEST(COALESCE(verified_until,now()),now())+interval '30 days', updated_at=now() WHERE user_id=$1 RETURNING verified_until`, uid).Scan(&until); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					fail(w, http.StatusBadRequest, "buka toko sebelum memakai kode VIP")
					return
				}
				fail(w, http.StatusInternalServerError, "gagal mengaktifkan VIP")
				return
			}
			out["verified_until"] = until
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE redeem_code SET used_count = used_count + 1,
			        redeemed_by = COALESCE(redeemed_by, $2),
			        redeemed_at = COALESCE(redeemed_at, now())
			  WHERE id=$1`, id, uid); err != nil {
			fail(w, http.StatusInternalServerError, "gagal memperbarui kode")
			return
		}
		if err := completeIdempotency(r.Context(), tx, uid, "redeem", requestKey, out); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mencatat idempotency request")
			return
		}
		if err := auditTx(r.Context(), tx, uid, "REDEEM", id, nil, map[string]any{"code": code, "amount": amount, "reward_type": rewardType}, ipOf(r)); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mencatat audit")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		ok(w, out)
	}
}

// AdminRedeemList: GET /api/admin/redeem.
func AdminRedeemList(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(),
			`SELECT id, code, amount, reward_type, max_uses, used_count, redeemed_by, redeemed_at, expires_at, created_at
			   FROM redeem_code ORDER BY created_at DESC LIMIT 200`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type code struct {
			ID         string     `json:"id"`
			Code       string     `json:"code"`
			Amount     float64    `json:"amount"`
			RewardType string     `json:"reward_type"`
			MaxUses    *int       `json:"max_uses,omitempty"`
			UsedCount  int        `json:"used_count"`
			RedeemedBy *string    `json:"redeemed_by,omitempty"`
			RedeemedAt *time.Time `json:"redeemed_at,omitempty"`
			ExpiresAt  *time.Time `json:"expires_at,omitempty"`
			CreatedAt  time.Time  `json:"created_at"`
		}
		out := []code{}
		for rows.Next() {
			var c code
			if err := rows.Scan(&c.ID, &c.Code, &c.Amount, &c.RewardType, &c.MaxUses, &c.UsedCount, &c.RedeemedBy, &c.RedeemedAt, &c.ExpiresAt, &c.CreatedAt); err != nil {
				continue
			}
			out = append(out, c)
		}
		ok(w, map[string]any{"redeems": out})
	}
}

// AdminRedeemCreate: POST /api/admin/redeem {amount, count, max_uses, expires_at}
// → kembalikan daftar kode yang dibuat (kode mentah hanya keluar di sini).
func AdminRedeemCreate(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			Amount     *float64   `json:"amount"`
			RewardType string     `json:"reward_type"`
			Count      int        `json:"count"`
			MaxUses    *int       `json:"max_uses"`
			ExpiresAt  *time.Time `json:"expires_at"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		if in.Amount == nil || *in.Amount <= 0 {
			fail(w, http.StatusBadRequest, "amount harus > 0")
			return
		}
		if in.RewardType == "" {
			in.RewardType = "WALLET"
		}
		if in.RewardType == "VERIFIED_30D" {
			*in.Amount = 30
		}
		if !redeemRewardAllowed(in.RewardType, *in.Amount) {
			fail(w, http.StatusBadRequest, "reward_type atau amount tidak valid")
			return
		}
		if in.MaxUses != nil && *in.MaxUses < 1 {
			fail(w, http.StatusBadRequest, "max_uses minimal 1")
			return
		}
		if in.Count <= 0 {
			in.Count = 1
		}
		if in.Count > 100 {
			fail(w, http.StatusBadRequest, "count maksimal 100")
			return
		}

		codes := []string{}
		for i := 0; i < in.Count; i++ {
			for attempt := 0; attempt < 5; attempt++ {
				c := genRedeemCode()
				res, err := db.Exec(r.Context(),
					`INSERT INTO redeem_code (code, amount, reward_type, max_uses, expires_at, created_by)
					 VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (code) DO NOTHING`,
					c, *in.Amount, in.RewardType, in.MaxUses, in.ExpiresAt, uid)
				if err != nil {
					fail(w, http.StatusInternalServerError, "gagal membuat kode")
					return
				}
				if res.RowsAffected() > 0 {
					codes = append(codes, c)
					break
				}
			}
		}
		audit(r.Context(), db, uid, "REDEEM_CREATE", "", nil,
			map[string]any{"count": len(codes), "amount": *in.Amount, "reward_type": in.RewardType, "max_uses": in.MaxUses}, ipOf(r))
		ok(w, map[string]any{"codes": codes, "amount": *in.Amount, "reward_type": in.RewardType})
	}
}

// AdminRedeemDelete: DELETE /api/admin/redeem/{id}.
func AdminRedeemDelete(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")
		if id == "" {
			id = r.URL.Query().Get("id")
		}
		res, err := db.Exec(r.Context(), `DELETE FROM redeem_code WHERE id=$1`, id)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal menghapus kode")
			return
		}
		if res.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "kode tidak ditemukan")
			return
		}
		audit(r.Context(), db, uid, "REDEEM_DELETE", id, nil, map[string]any{"id": id}, ipOf(r))
		ok(w, map[string]any{"ok": true})
	}
}

// genRedeemCode: "AISELL-XXXX-XXXX" — 8 karakter alfanumerik acak (32^8).
func genRedeemCode() string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // tanpa I/O/0/1: tidak ambigu
	b := make([]byte, 8)
	rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	s := string(b)
	return "AISELL-" + s[:4] + "-" + s[4:]
}
