package handler

import (
	"context"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"ai-sell/gateway/internal/store"
)

func TestDemoProductCannotCheckout(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji produk demo")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	var seller string
	if err := db.QueryRow(ctx,
		`INSERT INTO "user" (email, password_hash, wallet_balance) VALUES ($1,'x',0) RETURNING id`,
		"demo-seller-"+randCode(8)+"@example.invalid").Scan(&seller); err != nil {
		t.Fatalf("seed seller: %v", err)
	}
	t.Cleanup(func() { deleteFixture(t, ctx, db, seller) })
	buyer := seedUser(t, ctx, db)
	t.Cleanup(func() { deleteFixture(t, ctx, db, buyer) })
	if _, err := db.Exec(ctx, `UPDATE "user" SET wallet_balance=100 WHERE id=$1`, buyer); err != nil {
		t.Fatalf("seed buyer balance: %v", err)
	}

	var storeID, productID string
	if err := db.QueryRow(ctx, `INSERT INTO market_store (user_id, name) VALUES ($1,'Demo Store') RETURNING id`, seller).Scan(&storeID); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if err := db.QueryRow(ctx,
		`INSERT INTO market_product (seller_id, store_id, title, price, stock, is_active, is_demo)
		 VALUES ($1,$2,'Demo product',50,1,true,true) RETURNING id`, seller, storeID).Scan(&productID); err != nil {
		t.Fatalf("seed demo product: %v", err)
	}

	listRec := httptest.NewRecorder()
	ListProducts(db).ServeHTTP(listRec, httptest.NewRequest("GET", "/api/market/products", nil))
	if listRec.Code != 200 || !strings.Contains(listRec.Body.String(), `"is_demo":true`) {
		t.Fatalf("katalog demo harus tetap terlihat dengan flag is_demo: status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	res := checkoutOrder(httptest.NewRequest("POST", "/api/market/orders", nil), db, buyer, false,
		[]orderLine{{ProductID: productID, Qty: 1}}, "", "demo-"+randCode(12), func(orderResult) any { return nil })
	if res.Code != 400 || !strings.Contains(strings.ToLower(res.Reason), "demo") {
		t.Fatalf("checkout demo = %d %q, mau 400 dan alasan demo", res.Code, res.Reason)
	}

	var balance float64
	var stock int
	if err := db.QueryRow(ctx, `SELECT wallet_balance FROM "user" WHERE id=$1`, buyer).Scan(&balance); err != nil {
		t.Fatalf("baca saldo buyer: %v", err)
	}
	if err := db.QueryRow(ctx, `SELECT stock FROM market_product WHERE id=$1`, productID).Scan(&stock); err != nil {
		t.Fatalf("baca stok demo: %v", err)
	}
	if balance != 100 || stock != 1 {
		t.Fatalf("demo mengubah uang/stok: balance=%v stock=%d, mau 100/1", balance, stock)
	}
}

func TestListProductsVerifiedFilter(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji filter toko terverifikasi")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	verifiedSeller := seedUser(t, ctx, db)
	t.Cleanup(func() { deleteFixture(t, ctx, db, verifiedSeller) })
	plainSeller := seedUser(t, ctx, db)
	t.Cleanup(func() { deleteFixture(t, ctx, db, plainSeller) })
	var verifiedStore, plainStore string
	if err := db.QueryRow(ctx, `INSERT INTO market_store (user_id,name,is_verified,verified_until) VALUES ($1,'Verified Filter Store',true,now()+interval '1 day') RETURNING id`, verifiedSeller).Scan(&verifiedStore); err != nil {
		t.Fatalf("seed verified store: %v", err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO market_store (user_id,name) VALUES ($1,'Plain Filter Store') RETURNING id`, plainSeller).Scan(&plainStore); err != nil {
		t.Fatalf("seed plain store: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO market_product (seller_id,store_id,title,price,stock,is_active) VALUES ($1,$2,'Verified Filter Product',1,1,true),($3,$4,'Plain Filter Product',1,1,true)`, verifiedSeller, verifiedStore, plainSeller, plainStore); err != nil {
		t.Fatalf("seed products: %v", err)
	}
	rec := httptest.NewRecorder()
	ListProducts(db).ServeHTTP(rec, httptest.NewRequest("GET", "/api/market/products?verified=true", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Verified Filter Product") || strings.Contains(rec.Body.String(), "Plain Filter Product") {
		t.Fatalf("filter verified salah: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
