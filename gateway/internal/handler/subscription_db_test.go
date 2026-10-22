package handler

// Uji reconciler langganan terhadap DB sungguhan (dilewati kalau DATABASE_URL
// tidak ada). Tujuan: memastikan perpanjangan otomatis benar-benar jalan dan
// saldo kurang menghasilkan PAST_DUE — bukan diam-diam gagal.
import (
	"context"
	"os"
	"testing"
	"time"

	"ai-sell/gateway/internal/store"
)

func TestRenewDueSubscriptionsChargesWallet(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL tidak diset; lewati uji DB")
	}
	ctx := context.Background()
	pool, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("konek DB: %v", err)
	}
	defer pool.Close()

	// User + toko + langganan jatuh tempo dengan auto-renew, saldo cukup.
	email := "recon-" + time.Now().Format("150405.000000") + "@example.test"
	var uid string
	if err := pool.QueryRow(ctx,
		`INSERT INTO "user" (email, password_hash, role, wallet_balance, first_name, last_name)
		 VALUES ($1,'x','USER',50000,'Re','Con') RETURNING id`, email).Scan(&uid); err != nil {
		t.Fatalf("buat user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM subscription WHERE user_id=$1`, uid)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id=$1`, uid)
	})
	if _, err := pool.Exec(ctx,
		`INSERT INTO market_store (user_id,name,description,is_active) VALUES ($1,'Recon Store','uji',true)`, uid); err != nil {
		t.Fatalf("buat toko: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO subscription (user_id, tier_name, price, status, auto_renew, renew_days, renew_price, current_period_start, current_period_end)
		 VALUES ($1,'VERIFIED_STORE',15000,'ACTIVE',true,30,15000,now()-interval '31 days',now()-interval '1 hour')`, uid); err != nil {
		t.Fatalf("buat langganan: %v", err)
	}

	renewDueSubscriptions(ctx, pool)

	var saldo float64
	var status string
	if err := pool.QueryRow(ctx, `SELECT wallet_balance FROM "user" WHERE id=$1`, uid).Scan(&saldo); err != nil {
		t.Fatalf("baca saldo: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM subscription WHERE user_id=$1`, uid).Scan(&status); err != nil {
		t.Fatalf("baca status: %v", err)
	}
	if status != "ACTIVE" {
		t.Errorf("dengan saldo cukup, status harus ACTIVE; got %s", status)
	}
	if saldo != 35000 { // 50000 - 15000
		t.Errorf("saldo harus terpotong jadi 35000; got %.2f", saldo)
	}
}

func TestRenewDueSubscriptionsInsufficientBalanceBecomesPastDue(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL tidak diset; lewati uji DB")
	}
	ctx := context.Background()
	pool, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("konek DB: %v", err)
	}
	defer pool.Close()

	email := "recon0-" + time.Now().Format("150405.000000") + "@example.test"
	var uid string
	if err := pool.QueryRow(ctx,
		`INSERT INTO "user" (email, password_hash, role, wallet_balance, first_name, last_name)
		 VALUES ($1,'x','USER',0,'Re','Con') RETURNING id`, email).Scan(&uid); err != nil {
		t.Fatalf("buat user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM subscription WHERE user_id=$1`, uid)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id=$1`, uid)
	})
	if _, err := pool.Exec(ctx,
		`INSERT INTO market_store (user_id,name,description,is_active) VALUES ($1,'Recon Zero','uji',true)`, uid); err != nil {
		t.Fatalf("buat toko: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO subscription (user_id, tier_name, price, status, auto_renew, renew_days, renew_price, current_period_start, current_period_end)
		 VALUES ($1,'VERIFIED_STORE',15000,'ACTIVE',true,30,15000,now()-interval '31 days',now()-interval '1 hour')`, uid); err != nil {
		t.Fatalf("buat langganan: %v", err)
	}

	renewDueSubscriptions(ctx, pool)

	var saldo float64
	var status, lastErr string
	_ = pool.QueryRow(ctx, `SELECT wallet_balance FROM "user" WHERE id=$1`, uid).Scan(&saldo)
	if err := pool.QueryRow(ctx, `SELECT status, COALESCE(last_error,'') FROM subscription WHERE user_id=$1`, uid).Scan(&status, &lastErr); err != nil {
		t.Fatalf("baca langganan: %v", err)
	}
	// Inti: TIDAK ada pemotongan paksa → saldo tetap 0, bukan minus.
	if saldo != 0 {
		t.Errorf("saldo kurang TIDAK boleh dipotong paksa; saldo=%.2f", saldo)
	}
	if status != "PAST_DUE" {
		t.Errorf("saldo kurang harus jadi PAST_DUE; got %s", status)
	}
	if lastErr == "" {
		t.Error("PAST_DUE harus menyertakan alasan di last_error")
	}
}
