package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"ai-sell/gateway/internal/auth"
	"ai-sell/gateway/internal/provider"
	"ai-sell/gateway/internal/store"
)

// TestInstantItemsEncryptedAtRest: isi barang instan berisi kredensial, jadi
// baris di DB harus ciphertext (v1.) dan BuyerOrderItems harus mengembalikan
// plaintext aslinya. Lewat DB nyata karena inti klaimnya adalah "apa yang
// benar-benar tersimpan", bukan bentuk fungsi.
//
// Dilewati (bukan gagal) bila DATABASE_URL/SECRET_KEY belum diset supaya
// `go test ./...` tetap hijau tanpa stack.
func TestInstantItemsEncryptedAtRest(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" || len(os.Getenv("SECRET_KEY")) < 32 {
		t.Skip("DATABASE_URL/SECRET_KEY tidak diset — lewati uji isian instan")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	// Didaftarkan SEBELUM fixture: t.Cleanup LIFO, jadi pool baru ditutup
	// setelah deleteFixture jalan (kalau tidak: "closed pool" saat bersih-bersih).
	t.Cleanup(db.Close)

	uid := seedUser(t, ctx, db)
	t.Cleanup(func() { deleteFixture(t, ctx, db, uid) })

	plain := "rahasia-" + randCode(8) + "|p4ssw0rd"
	enc, err := encryptInstantItems([]string{plain, "kode-kedua-" + randCode(4)})
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if strings.Contains(enc[0], plain) {
		t.Fatal("ciphertext memuat plaintext")
	}

	var product, storeID string
	if err := db.QueryRow(ctx,
		`INSERT INTO market_store (user_id, name, logo) VALUES ($1,'Instan Test',$2) RETURNING id`,
		uid, "/api/media/"+uid).Scan(&storeID); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if err := db.QueryRow(ctx,
		`INSERT INTO market_product (seller_id, store_id, title, price, stock, is_active, is_instant)
		 VALUES ($1,$2,'Produk Instan',10,2,true,true) RETURNING id`, uid, storeID).Scan(&product); err != nil {
		t.Fatalf("seed produk: %v", err)
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO market_product_item (product_id, content)
		 SELECT $1, unnest($2::text[])`, product, enc); err != nil {
		t.Fatalf("simpan isian terenkripsi: %v", err)
	}

	// Di DB: tidak ada baris polos.
	var polos int
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM market_product_item WHERE product_id=$1 AND content NOT LIKE 'v1.%'`, product).
		Scan(&polos); err != nil {
		t.Fatalf("hitung baris polos: %v", err)
	}
	if polos != 0 {
		t.Fatalf("%d baris isian tersimpan tanpa enkripsi", polos)
	}

	// Order pembeli membawa kembali plaintext aslinya.
	var orderID string
	if err := db.QueryRow(ctx,
		`INSERT INTO market_order (buyer_id, product_id, store_id, qty, total_price, subtotal, status)
		 VALUES ($1,$2,$3,1,10,10,'PAID') RETURNING id`, uid, product, storeID).Scan(&orderID); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE market_product_item SET order_id=$1 WHERE product_id=$2`, orderID, product); err != nil {
		t.Fatalf("ikat isian: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/market/orders/"+orderID+"/items", nil)
	req.SetPathValue("id", orderID)
	req.Header.Set("Authorization", "Bearer "+sessionFor(t, ctx, db, uid))
	rec := httptest.NewRecorder()
	// Lewat RequireAuth asli: token → hash → user_session → uid di context.
	auth.RequireAuth(db)(BuyerOrderItems(db)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("BuyerOrderItems: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), plain) {
		t.Fatalf("plaintext tidak kembali ke pembeli: %s", rec.Body.String())
	}
}

// TestServeMediaPublicWhenReferenced: logo toko & gambar produk menunjuk
// /api/media/<id>; pengunjung tanpa sesi harus bisa memuatnya. Aset yang tidak
// dirujuk baris publik tetap 404 untuk bukan pemilik.
func TestServeMediaPublicWhenReferenced(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji media publik")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(db.Close)

	owner := seedUser(t, ctx, db)
	t.Cleanup(func() { deleteFixture(t, ctx, db, owner) })

	var publicID, privateID, bannerID string
	png := []byte("\x89PNG\r\n\x1a\n")
	if err := db.QueryRow(ctx,
		`INSERT INTO media_asset (user_id, file_name, mime_type, bytes, data)
		 VALUES ($1,'logo.png','image/png', octet_length($2::bytea), $2::bytea) RETURNING id`, owner, png).Scan(&publicID); err != nil {
		t.Fatalf("seed media publik: %v", err)
	}
	if err := db.QueryRow(ctx,
		`INSERT INTO media_asset (user_id, file_name, mime_type, bytes, data)
		 VALUES ($1,'privat.png','image/png', octet_length($2::bytea), $2::bytea) RETURNING id`, owner, png).Scan(&privateID); err != nil {
		t.Fatalf("seed media privat: %v", err)
	}
	if err := db.QueryRow(ctx,
		`INSERT INTO media_asset (user_id, file_name, mime_type, bytes, data)
		 VALUES ($1,'banner.png','image/png', octet_length($2::bytea), $2::bytea) RETURNING id`, owner, png).Scan(&bannerID); err != nil {
		t.Fatalf("seed media banner: %v", err)
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO market_store (user_id, name, logo) VALUES ($1,'Media Test',$2)`,
		owner, "/api/media/"+publicID); err != nil {
		t.Fatalf("seed store logo: %v", err)
	}

	get := func(id string) int {
		req := httptest.NewRequest("GET", "/api/media/"+id, nil)
		req.SetPathValue("id", id)
		rec := httptest.NewRecorder()
		ServeMedia(db)(rec, req)
		return rec.Code
	}
	if code := get(publicID); code != http.StatusOK {
		t.Fatalf("logo toko dirujuk publik tapi %d, mau 200", code)
	}
	if code := get(privateID); code != http.StatusNotFound {
		t.Fatalf("aset privat untuk anonim = %d, mau 404", code)
	}
	var oldBanners string
	_ = db.QueryRow(ctx, `SELECT value FROM platform_setting WHERE key='market_banners'`).Scan(&oldBanners)
	t.Cleanup(func() {
		_, _ = db.Exec(ctx, `UPDATE platform_setting SET value=$1 WHERE key='market_banners'`, oldBanners)
	})
	if _, err := db.Exec(ctx, `INSERT INTO platform_setting(key,value) VALUES ('market_banners',$1)
		ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`, `[{"active":true,"image_url":"/api/media/`+bannerID+`"}]`); err != nil {
		t.Fatalf("seed banner setting: %v", err)
	}
	if code := get(bannerID); code != http.StatusOK {
		t.Fatalf("banner marketplace dirujuk publik tapi %d, mau 200", code)
	}

	// Orang lain tidak boleh mempublikasikan berkas milik seller ini dengan
	// merujuknya dari produk/toko sendiri.
	stranger := seedUser(t, ctx, db)
	t.Cleanup(func() { deleteFixture(t, ctx, db, stranger) })
	var strangerStore string
	if err := db.QueryRow(ctx,
		`INSERT INTO market_store (user_id, name) VALUES ($1,'Penyerobot') RETURNING id`, stranger).Scan(&strangerStore); err != nil {
		t.Fatalf("seed toko penyerobot: %v", err)
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO market_product (seller_id, store_id, title, price, stock, is_active, image_url)
		 VALUES ($1,$2,'Curi Gambar',10,1,true,$3)`, stranger, strangerStore, "/api/media/"+privateID); err != nil {
		t.Fatalf("seed produk penyerobot: %v", err)
	}
	if code := get(privateID); code != http.StatusNotFound {
		t.Fatalf("berkas privat dipublikasikan orang lain = %d, mau 404", code)
	}
}

// sessionFor: token + baris user_session yang sah, supaya jalur auth-nya
// (Bearer → HashKey → user_session) benar-benar diuji, bukan diterabas.
func sessionFor(t *testing.T, ctx context.Context, db DB, uid string) string {
	t.Helper()
	tok, err := auth.IssueToken(uid, "USER")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO user_session (session_token, user_id, expires) VALUES ($1,$2, now() + interval '1 hour')`,
		provider.HashKey(tok), uid); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return tok
}
