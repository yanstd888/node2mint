package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"ai-sell/gateway/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Uji Marketplace pilihan: semua produk admin masuk; seller biasa hanya masuk
// saat boost-nya masih aktif. Seller biasa dengan penjualan tinggi tetap bukan
// Marketplace pilihan.
//
// Dilewati bila DATABASE_URL tidak diset supaya `go test ./...` tetap hijau
// tanpa DB. Fixture milik tes ini saja; dihapus lewat DELETE user (CASCADE).
func TestHomeFeaturedPinnedAcrossStores(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji urutan homepage")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	adminID, adminStore := seedAdminStoreFixture(t, ctx, db)
	defer deleteFixture(t, ctx, db, adminID)

	sellerID, sellerStore := seedSoftDeleteFixture(t, ctx, db)
	defer deleteFixture(t, ctx, db, sellerID)

	top := seedSoldProduct(t, ctx, db, adminID, adminStore, "featured admin 1", 5, false, 1, "2026-01-01")
	mid := seedSoldProduct(t, ctx, db, adminID, adminStore, "featured admin 2", 2, false, 2, "2026-02-01")
	boosted := seedSoldProduct(t, ctx, db, adminID, adminStore, "featured admin 3", 0, true, 3, "2026-05-01")
	newest := seedSoldProduct(t, ctx, db, adminID, adminStore, "featured admin 4", 0, false, 4, "2026-06-01")
	seller := seedSoldProduct(t, ctx, db, sellerID, sellerStore, "featured seller 5", 0, true, 5, "2026-07-01")
	ordinary := seedSoldProduct(t, ctx, db, sellerID, sellerStore, "ordinary seller 99", 99, false, 6, "2026-08-01")

	rec := httptest.NewRecorder()
	HomeFeatured(db)(rec, httptest.NewRequest(http.MethodGet, "/api/market/featured?per_page=5", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, mau 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Products []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body.String())
	}
	if len(body.Products) != 5 {
		t.Fatalf("dapat %d produk, mau tepat 5", len(body.Products))
	}
	want := []string{top, mid, boosted, newest, seller}
	for i, w := range want {
		if body.Products[i].ID != w {
			t.Errorf("urutan[%d]=%q (%s), mau %q", i, body.Products[i].ID, body.Products[i].Title, w)
		}
	}
	for _, p := range body.Products {
		if p.ID == "" {
			t.Errorf("produk Featured tanpa id: %s", p.Title)
		}
		if p.ID == ordinary {
			t.Errorf("seller biasa masuk Marketplace pilihan: %s", p.Title)
		}
	}

	bestRec := httptest.NewRecorder()
	ListProducts(db)(bestRec, httptest.NewRequest(http.MethodGet, "/api/market/products?sort=best_selling&available=true&per_page=10", nil))
	if bestRec.Code != http.StatusOK {
		t.Fatalf("best seller status %d, mau 200 (body=%s)", bestRec.Code, bestRec.Body.String())
	}
	var bestBody struct {
		Products []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"products"`
	}
	if err := json.Unmarshal(bestRec.Body.Bytes(), &bestBody); err != nil {
		t.Fatalf("decode best seller: %v (body=%s)", err, bestRec.Body.String())
	}
	if len(bestBody.Products) != 5 {
		t.Fatalf("best seller dapat %d produk, mau 5", len(bestBody.Products))
	}
	seen := map[string]bool{}
	for _, p := range bestBody.Products {
		seen[p.ID] = true
		if p.ID == ordinary {
			t.Errorf("seller biasa masuk Produk terlaris: %s", p.Title)
		}
	}
	for _, id := range []string{top, mid, boosted, newest, seller} {
		if !seen[id] {
			t.Errorf("produk admin/boost hilang dari Produk terlaris: %s", id)
		}
	}
}

// seedAdminStoreFixture: user role ADMIN + toko miliknya.
func seedAdminStoreFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) (owner, storeID string) {
	t.Helper()
	email := "featured-admin-test-" + randCode(6) + "@example.invalid"
	if err := db.QueryRow(ctx,
		`INSERT INTO "user" (email, password_hash, wallet_balance, role) VALUES ($1,'x',0,'ADMIN') RETURNING id`,
		email).Scan(&owner); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	if err := db.QueryRow(ctx,
		`INSERT INTO market_store (user_id, name) VALUES ($1,'Test Featured Admin') RETURNING id`,
		owner).Scan(&storeID); err != nil {
		t.Fatalf("seed store admin: %v", err)
	}
	return owner, storeID
}

func seedSoldProduct(t *testing.T, ctx context.Context, db *pgxpool.Pool, owner, storeID, title string, sales int, boosted bool, pin int, created string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(ctx,
		`INSERT INTO market_product (seller_id, store_id, title, description, price, stock, is_active, sales_count, is_boosted, boosted_until, pin_order, created_at)
		 VALUES ($1,$2,$3,'uji urutan homepage',1000,5,true,$4,$5,CASE WHEN $5 THEN now()+interval '6 hours' ELSE NULL END,$6,$7::timestamptz) RETURNING id`,
		owner, storeID, title, sales, boosted, pin, created).Scan(&id); err != nil {
		t.Fatalf("seed produk %q: %v", title, err)
	}
	return id
}

// setAdminStoreEmail menulis platform_setting, mengembalikan fungsi pemulih.
func setAdminStoreEmail(t *testing.T, ctx context.Context, db *pgxpool.Pool) func() {
	t.Helper()
	var old string
	hadOld := db.QueryRow(ctx, `SELECT value FROM platform_setting WHERE key='admin_store_user_email'`).Scan(&old) == nil
	var email string
	if err := db.QueryRow(ctx, `SELECT email FROM "user" WHERE role='ADMIN' ORDER BY created_at DESC LIMIT 1`).Scan(&email); err != nil {
		t.Fatalf("baca email admin: %v", err)
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO platform_setting (key, value, updated_at) VALUES ('admin_store_user_email',$1,now())
		 ON CONFLICT (key) DO UPDATE SET value=$1, updated_at=now()`, email); err != nil {
		t.Fatalf("set setting: %v", err)
	}
	return func() {
		if hadOld {
			_, _ = db.Exec(ctx, `UPDATE platform_setting SET value=$1 WHERE key='admin_store_user_email'`, old)
			return
		}
		_, _ = db.Exec(ctx, `DELETE FROM platform_setting WHERE key='admin_store_user_email'`)
	}
}
