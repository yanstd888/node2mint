package handler

import (
	"context"
	"os"
	"testing"

	"ai-sell/gateway/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Uji hapus lunak seller: hapus TIDAK membuang baris (pesanan/keranjang/link
// tetap menunjuk produk_id), produk keluar dari katalog publik, dan restore
// memunculkannya lagi. Dilewati bila DATABASE_URL tidak diset — `go test ./...`
// tetap hijau tanpa DB.
//
// Fixture hanya milik tes ini (email acak) dan dibersihkan lewat DELETE user
// (ON DELETE CASCADE menghapus store + produk).
func TestProductSoftDeleteHidesAndRestores(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji hapus lunak")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	owner, storeID := seedSoftDeleteFixture(t, ctx, db)
	defer deleteFixture(t, ctx, db, owner)

	productID := seedProduct(t, ctx, db, owner, storeID, "Produk Hapus Uji")

	// 1) Hapus lunak: baris tetap ada, hilang dari katalog publik.
	res, err := db.Exec(ctx, productSoftDeleteSQL, productID, storeID)
	if err != nil {
		t.Fatalf("hapus lunak: %v", err)
	}
	if res.RowsAffected() != 1 {
		t.Fatalf("hapus lunak mengubah %d baris, mau 1", res.RowsAffected())
	}
	if !exists(ctx, db, `SELECT true FROM market_product WHERE id=$1`, productID) {
		t.Fatal("baris produk ikut TERHAPUS — riwayat pesanan akan hilang")
	}
	if visible(ctx, db, productID) {
		t.Fatal("produk terhapus masih muncul di katalog publik")
	}
	if exists(ctx, db, `SELECT true FROM market_product WHERE id=$1 AND is_active`, productID) {
		t.Fatal("produk terhapus masih is_active=true")
	}

	// 2) Hapus ulang = idempoten: 0 baris berubah, bukan error.
	res, err = db.Exec(ctx, productSoftDeleteSQL, productID, storeID)
	if err != nil {
		t.Fatalf("hapus ulang: %v", err)
	}
	if res.RowsAffected() != 0 {
		t.Fatalf("hapus ulang mengubah %d baris, mau 0", res.RowsAffected())
	}

	// 3) Restore (Undo): muncul lagi di katalog, exactly once.
	res, err = db.Exec(ctx, productRestoreSQL, productID, storeID)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if res.RowsAffected() != 1 {
		t.Fatalf("restore mengubah %d baris, mau 1", res.RowsAffected())
	}
	if !visible(ctx, db, productID) {
		t.Fatal("produk tidak kembali ke katalog setelah restore")
	}

	// 4) Restore pada produk yang tidak terhapus = 0 baris (404 di handler).
	res, err = db.Exec(ctx, productRestoreSQL, productID, storeID)
	if err != nil {
		t.Fatalf("restore kedua: %v", err)
	}
	if res.RowsAffected() != 0 {
		t.Fatalf("restore produk aktif mengubah %d baris, mau 0", res.RowsAffected())
	}
}

// TestProductSoftDeleteRefusedWhenOrdered: produk yang sudah punya pesanan
// tidak boleh dihapus lunak — riwayat pesanan tetap utuh, jawaban handler 409.
func TestProductSoftDeleteRefusedWhenOrdered(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji hapus lunak")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	owner, storeID := seedSoftDeleteFixture(t, ctx, db)
	defer deleteFixture(t, ctx, db, owner)

	productID := seedProduct(t, ctx, db, owner, storeID, "Produk Berpesanan Uji")
	buyer := seedUser(t, ctx, db)
	defer deleteFixture(t, ctx, db, buyer)
	if _, err := db.Exec(ctx,
		`INSERT INTO market_order (buyer_id, product_id, store_id, qty, total_price, subtotal)
		 VALUES ($1,$2,$3,1,1000,1000)`, buyer, productID, storeID); err != nil {
		t.Fatalf("seed order: %v", err)
	}

	res, err := db.Exec(ctx, productSoftDeleteSQL, productID, storeID)
	if err != nil {
		t.Fatalf("hapus lunak produk berpesanan: %v", err)
	}
	if res.RowsAffected() != 0 {
		t.Fatalf("produk berpesanan terhapus lunak (%d baris) — seharusnya ditolak 409", res.RowsAffected())
	}
	if !visible(ctx, db, productID) {
		t.Fatal("produk berpesanan hilang dari katalog padahal hapus seharusnya ditolak")
	}
}

func seedSoftDeleteFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) (owner, storeID string) {
	t.Helper()
	email := "softdelete-test-" + randCode(6) + "@example.invalid"
	if err := db.QueryRow(ctx,
		`INSERT INTO "user" (email, password_hash, wallet_balance) VALUES ($1,'x',0) RETURNING id`,
		email).Scan(&owner); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.QueryRow(ctx,
		`INSERT INTO market_store (user_id, name) VALUES ($1,'Test Hapus Lunak') RETURNING id`,
		owner).Scan(&storeID); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	return owner, storeID
}

func seedProduct(t *testing.T, ctx context.Context, db *pgxpool.Pool, owner, storeID, title string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(ctx,
		`INSERT INTO market_product (seller_id, store_id, title, description, price, stock, is_active)
		 VALUES ($1,$2,$3,'deskripsi uji hapus lunak',1000,5,true) RETURNING id`,
		owner, storeID, title).Scan(&id); err != nil {
		t.Fatalf("seed produk: %v", err)
	}
	return id
}

// visible = produk muncul di katalog publik (syarat yang sama dengan ListProducts).
func visible(ctx context.Context, db *pgxpool.Pool, productID string) bool {
	return exists(ctx, db,
		`SELECT true FROM market_product WHERE id=$1 AND is_active = true AND deleted_at IS NULL`, productID)
}

func exists(ctx context.Context, db *pgxpool.Pool, sql string, args ...any) bool {
	var b bool
	return db.QueryRow(ctx, sql, args...).Scan(&b) == nil
}
