package handler

import (
	"context"
	"errors"
	"os"
	"testing"

	"ai-sell/gateway/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Uji jalur uang: saldo toko dipotong HANYA bila cukup, dompet pemilik naik
// persis jumlah yang sama, dan ada satu baris buku besar. Dilewati (bukan
// gagal) bila DATABASE_URL tidak diset supaya `go test ./...` tetap hijau.
//
// Tidak ada reset DB: hanya user/store sementara milik tes ini yang dibuat,
// dan dihapus lagi lewat DELETE (ON DELETE CASCADE menghapus toko + mutasi).
func TestTransferStoreWallet(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji jalur uang")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	owner, storeID := seedWalletFixture(t, ctx, db, 1000)
	defer deleteFixture(t, ctx, db, owner)

	// 1) Saldo cukup: 500 pindah, tidak lebih tidak kurang.
	if got, err := transferStoreWallet(ctx, db, owner, 500, ""); err != nil || got != storeID {
		t.Fatalf("transfer valid: store=%q err=%v, mau store=%q", got, err, storeID)
	}
	if s, u := balances(t, ctx, db, owner); s != 500 || u != 500 {
		t.Fatalf("saldo setelah transfer: toko=%v user=%v, mau 500/500", s, u)
	}
	var kind string
	var amount float64
	if err := db.QueryRow(ctx,
		`SELECT type, amount FROM credit_transaction WHERE user_id=$1`, owner).Scan(&kind, &amount); err != nil {
		t.Fatalf("buku besar tidak tercatat: %v", err)
	}
	if kind != "STORE_TRANSFER" || amount != 500 {
		t.Fatalf("mutasi salah: type=%s amount=%v", kind, amount)
	}

	// 2) Saldo tidak cukup: mustahil sebagian — kedua sisi harus tetap utuh.
	err = nil
	if _, err = transferStoreWallet(ctx, db, owner, 1_000_000, ""); !errors.Is(err, errInsufficientStoreFund) {
		t.Fatalf("transfer melebihi saldo: err=%v, mau errInsufficientStoreFund", err)
	}
	if s, u := balances(t, ctx, db, owner); s != 500 || u != 500 {
		t.Fatalf("saldo berubah walau transfer ditolak: toko=%v user=%v", s, u)
	}
	var rows int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM credit_transaction WHERE user_id=$1`, owner).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("baris buku besar setelah penolakan: %d (err=%v), mau 1", rows, err)
	}

	// 3) Tanpa toko: ditolak sebagai errNoStore, bukan errInsufficientStoreFund.
	stranger := seedUser(t, ctx, db)
	defer deleteFixture(t, ctx, db, stranger)
	if _, err := transferStoreWallet(ctx, db, stranger, 1, ""); !errors.Is(err, errNoStore) {
		t.Fatalf("user tanpa toko: err=%v, mau errNoStore", err)
	}

	// 4) Nominal nol/negatif bukan urusan fungsi ini (dijaga handler), tapi
	// pembagian errNoStore vs errInsufficientStoreFund yang di atas sudah cukup.
}

func seedUser(t *testing.T, ctx context.Context, db *pgxpool.Pool) string {
	t.Helper()
	email := "wallet-nostore-test-" + randCode(8) + "@example.invalid"
	var uid string
	if err := db.QueryRow(ctx,
		`INSERT INTO "user" (email, password_hash, wallet_balance) VALUES ($1,'x',0) RETURNING id`,
		email).Scan(&uid); err != nil {
		t.Fatalf("seed user tanpa toko: %v", err)
	}
	return uid
}

// seedWalletFixture membuat user + toko sementara dengan saldo awal.
func seedWalletFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool, storeBalance float64) (owner, storeID string) {
	t.Helper()
	email := "wallet-transfer-test-" + randCode(8) + "@example.invalid"
	if err := db.QueryRow(ctx,
		`INSERT INTO "user" (email, password_hash, wallet_balance) VALUES ($1,'x',0) RETURNING id`,
		email).Scan(&owner); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.QueryRow(ctx,
		`INSERT INTO market_store (user_id, name, wallet_balance) VALUES ($1,'Test Transfer',$2) RETURNING id`,
		owner, storeBalance).Scan(&storeID); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	return owner, storeID
}

func deleteFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool, uid string) {
	t.Helper()
	if _, err := db.Exec(ctx, `DELETE FROM "user" WHERE id=$1`, uid); err != nil {
		t.Fatalf("bersihkan fixture: %v", err)
	}
}

func balances(t *testing.T, ctx context.Context, db *pgxpool.Pool, uid string) (storeBal, userBal float64) {
	t.Helper()
	if err := db.QueryRow(ctx,
		`SELECT (SELECT wallet_balance FROM market_store WHERE user_id=$1),
		        (SELECT wallet_balance FROM "user" WHERE id=$1)`, uid).Scan(&storeBal, &userBal); err != nil {
		t.Fatalf("baca saldo: %v", err)
	}
	return storeBal, userBal
}
