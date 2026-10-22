// Billing: atomic credit check + deduct + usage log via satu Postgres transaction.
// Saldo tidak boleh pernah minus karena constraint CHECK di table.
//
// Jejak usage harus jujur. ChargeTx menulis baris usage_log dengan status 'OK'
// dan biaya penuh SEBELUM provider dipanggil, jadi satu-satunya tempat yang
// boleh membatalkannya adalah SettleUsage() di file ini — bukan tambalan
// "update usage_log" di tiap handler yang memanggil provider.
package billing

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrInsufficientCredit menandai saldo kurang, bukan kegagalan teknis.
var ErrInsufficientCredit = errors.New("insufficient_credit")

// ErrCreditLimitExceeded: kuota per API key (credit_limit) tercapai — keputusan
// bisnis seperti saldo kurang, bukan kegagalan teknis.
var ErrCreditLimitExceeded = errors.New("credit_limit_exceeded")

type ChargeResult struct {
	Remaining float64 `json:"remaining"`
	// UsageID = baris usage_log yang dibuat pra-potong; dipakai SettleUsage.
	UsageID string `json:"usage_id"`
}

// Charge membungkus ChargeTx dalam transaksi baru untuk operasi mandiri.
func Charge(ctx context.Context, db *pgxpool.Pool, userID, apiKeyID, model string, cost float64, input, output int) (ChargeResult, error) {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ChargeResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := ChargeTx(ctx, tx, userID, apiKeyID, model, cost, input, output)
	if err != nil {
		return ChargeResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ChargeResult{}, fmt.Errorf("commit: %w", err)
	}
	return result, nil
}

// SettleUsage menutup satu baris usage_log setelah provider menjawab:
// status akhir, biaya & token sebenarnya, dan pengembalian selisih ke paket token.
// Pemanggil tidak perlu tahu soal selisih — Cuma status "OK"/"ERROR".
//
// in/out < 0 = jangan ubah token yang sudah tercatat; cache < 0 sama.
func SettleUsage(ctx context.Context, db *pgxpool.Pool, userID, usageID, status string, charged, actual float64, in, out, cache int) error {
	if usageID == "" {
		return fmt.Errorf("usage_id kosong")
	}
	if actual < 0 {
		actual = 0
	}
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin settle: %w", err)
	}
	defer tx.Rollback(ctx)
	chargedUnits, actualUnits := tokenUnits(charged), tokenUnits(actual)
	if chargedUnits > actualUnits {
		if err := refundTokenPackage(ctx, tx, userID, chargedUnits-actualUnits); err != nil {
			return err
		}
	} else if actualUnits > chargedUnits {
		taken, err := takeTokenPackages(ctx, tx, userID, actualUnits-chargedUnits)
		if err != nil {
			return err
		}
		// Provider sudah menjawab; jika estimasi input kurang dan token habis,
		// platform menanggung sisanya. Dompet tidak pernah menjadi fallback.
		actualUnits = chargedUnits + taken
	}

	if _, err := tx.Exec(ctx, `UPDATE usage_log SET status=$1, cost=$2,
			input_tokens  = CASE WHEN $3 < 0 THEN input_tokens  ELSE $3 END,
			output_tokens = CASE WHEN $4 < 0 THEN output_tokens ELSE $4 END,
			cache_tokens  = CASE WHEN $6 < 0 THEN cache_tokens  ELSE $6 END
		WHERE id = $5`, status, float64(actualUnits), in, out, usageID, cache); err != nil {
		return fmt.Errorf("settle usage_log: %w", err)
	}
	// Kuota per key: selisih estimasi vs biaya nyata dipulihkan juga dari
	// credit_used key terkait. usage_log tanpa api_key_id = tidak ada efek.
	if _, err := tx.Exec(ctx, `UPDATE api_key SET credit_used = GREATEST(0, credit_used - $1)
		WHERE id = (SELECT api_key_id FROM usage_log WHERE id=$2) AND $1 <> 0`, float64(chargedUnits-actualUnits), usageID); err != nil {
		return fmt.Errorf("adjust credit_used: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit settle: %w", err)
	}
	return nil
}

// RefundTx mengembalikan saldo di dalam transaksi pemanggil, supaya refund
// order (mis. batal marketplace) ikut satu transaksi dengan perubahan stok.
func RefundTx(ctx context.Context, tx pgx.Tx, userID string, amount float64, description string) error {
	if amount <= 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE "user" SET wallet_balance = wallet_balance + $1 WHERE id = $2`, amount, userID); err != nil {
		return fmt.Errorf("refund balance: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO credit_transaction (user_id, amount, type, status, description)
		 VALUES ($1, $2, 'REFUND', 'COMPLETED', $3)`, userID, amount, description); err != nil {
		return fmt.Errorf("refund transaction: %w", err)
	}
	return nil
}

func tokenUnits(cost float64) int64 {
	if cost <= 0 {
		return 0
	}
	return int64(math.Ceil(cost))
}

func takeTokenPackages(ctx context.Context, tx pgx.Tx, userID string, units int64) (int64, error) {
	if units <= 0 {
		return 0, nil
	}
	rows, err := tx.Query(ctx, `SELECT id, remaining FROM user_token_package
		WHERE user_id=$1 AND expires_at > now() AND remaining > 0
		ORDER BY requires_telegram ASC, expires_at, id FOR UPDATE`, userID)
	if err != nil {
		return 0, fmt.Errorf("lock paket token: %w", err)
	}
	type tokenPackage struct {
		id        string
		remaining int64
	}
	var packages []tokenPackage
	for rows.Next() {
		var p tokenPackage
		if err := rows.Scan(&p.id, &p.remaining); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan paket token: %w", err)
		}
		packages = append(packages, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, fmt.Errorf("baca paket token: %w", err)
	}
	taken := int64(0)
	for _, p := range packages {
		if taken >= units {
			break
		}
		use := min(p.remaining, units-taken)
		if _, err := tx.Exec(ctx, `UPDATE user_token_package
			SET remaining=remaining-$2, used_amount=used_amount+$2 WHERE id=$1`, p.id, use); err != nil {
			return 0, fmt.Errorf("pakai paket token: %w", err)
		}
		taken += use
	}
	return taken, nil
}

func refundTokenPackage(ctx context.Context, tx pgx.Tx, userID string, units int64) error {
	if units <= 0 {
		return nil
	}
	tag, err := tx.Exec(ctx, `UPDATE user_token_package
		SET remaining=remaining+$1, used_amount=GREATEST(0,used_amount-$1)
		WHERE id=(SELECT id FROM user_token_package WHERE user_id=$2 ORDER BY expires_at LIMIT 1 FOR UPDATE)`, units, userID)
	if err != nil {
		return fmt.Errorf("kembalikan paket token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("paket token asal tidak ditemukan")
	}
	return nil
}

// ChargeTx hanya memakai paket token API. Dompet adalah uang untuk pembelian,
// bukan fallback penggunaan AI.
func ChargeTx(ctx context.Context, tx pgx.Tx, userID, apiKeyID, model string, cost float64, input, output int) (ChargeResult, error) {
	if cost < 0 {
		return ChargeResult{}, fmt.Errorf("cost must not be negative")
	}

	units := tokenUnits(cost)
	taken, err := takeTokenPackages(ctx, tx, userID, units)
	if err != nil {
		return ChargeResult{}, err
	}
	if taken < units {
		return ChargeResult{}, ErrInsufficientCredit
	}

	// Kuota per API key (port new-api): cek & naikkan credit_used di transaksi
	// yang sama. credit_limit NULL = tanpa batas.
	if apiKeyID != "" {
		var used float64
		var limit *float64
		if err := tx.QueryRow(ctx,
			`SELECT credit_used, credit_limit FROM api_key WHERE id = $1 FOR UPDATE`,
			apiKeyID).Scan(&used, &limit); err != nil {
			return ChargeResult{}, fmt.Errorf("lock api_key: %w", err)
		}
		if limit != nil && used+float64(units) > *limit {
			return ChargeResult{}, ErrCreditLimitExceeded
		}
		if _, err := tx.Exec(ctx, `UPDATE api_key SET credit_used = credit_used + $1 WHERE id = $2`,
			float64(units), apiKeyID); err != nil {
			return ChargeResult{}, fmt.Errorf("update credit_used: %w", err)
		}
	}

	var key any
	if apiKeyID != "" {
		key = apiKeyID
	}
	var usageID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO usage_log (user_id, api_key_id, model, input_tokens, output_tokens, cost, status)
		 VALUES ($1, $2, $3, $4, $5, $6, 'OK') RETURNING id`,
		userID, key, model, input, output, float64(units)).Scan(&usageID); err != nil {
		return ChargeResult{}, fmt.Errorf("insert usage_log: %w", err)
	}
	var remaining float64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(remaining),0) FROM user_token_package
		WHERE user_id=$1 AND expires_at>now()`, userID).Scan(&remaining); err != nil {
		return ChargeResult{}, fmt.Errorf("baca sisa token: %w", err)
	}
	return ChargeResult{Remaining: remaining, UsageID: usageID}, nil
}
