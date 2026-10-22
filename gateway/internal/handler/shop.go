package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"ai-sell/gateway/internal/auth"
	"ai-sell/gateway/internal/provider"
	"ai-sell/gateway/internal/secret"

	"github.com/jackc/pgx/v5"
)

// Toko, ulasan, unggahan media, katalog model, status publik, dan artikel.
//
// ponytail: berkas terpisah dari commerce.go supaya batasnya jelas — commerce =
// uang, shop = konten katalog. Digabung jadi satu berkas 900 baris tidak
// menambah kejelasan.

// ------------------------------------------------------------------ TOKO

type storeInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Desc        string    `json:"description"`
	Logo        string    `json:"logo,omitempty"`
	IsActive    bool      `json:"is_active"`
	IsVerified  bool      `json:"is_verified"`
	IsAdmin     bool      `json:"is_admin"`
	Balance     float64   `json:"wallet_balance"`
	CreatedAt   time.Time `json:"created_at"`
	Products    int       `json:"products"`
	Orders      int       `json:"orders"`
	Rating      float64   `json:"rating,omitempty"`
	ReviewCount int       `json:"review_count,omitempty"`
	BoostTill   string    `json:"boost_until,omitempty"`
	// verified_until dikirim apa adanya: klien perlu tahu SISA masa verified
	// (bukan cuma true/false) supaya bisa menampilkan durasi dan tombol perpanjang.
	VerifiedUntil string `json:"verified_until,omitempty"`
	// Nilai mentah dari DB; tidak di-JSON-kan. Dipakai loadStore untuk memutuskan
	// apakah VerifiedUntil masih relevan untuk ditampilkan.
	VerifiedUntilRaw *time.Time `json:"-"`
	// Harga dikirim dari server supaya popup konfirmasi menampilkan angka yang
	// benar-benar akan dipotong dari dompet, bukan angka hardcode di UI.
	BoostPricePerDay   float64 `json:"boost_price_per_day"`
	BoostPriceOneOff   float64 `json:"boost_price_one_off"`
	BoostMaxPerDay     int     `json:"boost_max_per_day"`
	VerifiedMonthlyIDR float64 `json:"verified_monthly_price"`
	BoostCredits       int     `json:"boost_credits"`
	FreeBoostLimit     int     `json:"free_boost_limit"`
}

func loadStore(ctx context.Context, db DB, uid string) (storeInfo, bool) {
	var s storeInfo
	err := db.QueryRow(ctx,
		`SELECT s.id, s.name, COALESCE(s.description,''), s.is_active, (s.is_verified AND (s.verified_until IS NULL OR s.verified_until > now())), (u.role IN ('ADMIN','SUPER_ADMIN')), s.wallet_balance, s.created_at, COALESCE(s.logo,''), s.verified_until, s.boost_credits
		   FROM market_store s JOIN "user" u ON u.id=s.user_id WHERE s.user_id=$1`, uid).
		Scan(&s.ID, &s.Name, &s.Desc, &s.IsActive, &s.IsVerified, &s.IsAdmin, &s.Balance, &s.CreatedAt, &s.Logo, &s.VerifiedUntilRaw, &s.BoostCredits)
	if err != nil {
		// Jangan samarkan kesalahan query sebagai "belum punya toko"; caller dapat
		// tetap mengembalikan null, tetapi log ini menjaga diagnosis data/scan gagal.
		log.Printf("[WARN] loadStore uid=%s: %v", uid, err)
		return storeInfo{}, false
	}
	_ = db.QueryRow(ctx, `SELECT COUNT(*) FROM market_product WHERE store_id=$1`, s.ID).Scan(&s.Products)
	_ = db.QueryRow(ctx, `SELECT COUNT(*) FROM market_order WHERE store_id=$1`, s.ID).Scan(&s.Orders)
	var till *time.Time
	if err := db.QueryRow(ctx,
		`SELECT MAX(expires_at) FROM market_boost WHERE store_id=$1 AND expires_at > now()`, s.ID).Scan(&till); err == nil && till != nil {
		s.BoostTill = till.Format(time.RFC3339)
	}
	// Sisa masa verified: hanya dikirim kalau masih berlaku (masa lalu tidak
	// berguna buat UI dan bikin bingung).
	if s.VerifiedUntilRaw != nil && s.VerifiedUntilRaw.After(time.Now()) {
		s.VerifiedUntil = s.VerifiedUntilRaw.Format(time.RFC3339)
	}
	// Harga efektif dari platform_setting — sama persis dengan yang dipakai
	// handler boost/verify saat memotong dompet.
	s.BoostPricePerDay = settingFloat(ctx, db, "boost_price_per_day", 5000)
	s.BoostPriceOneOff = settingFloat(ctx, db, "boost_price_one_off", 1000)
	s.BoostMaxPerDay = int(settingFloat(ctx, db, "boost_max_verified_per_day", 2))
	s.VerifiedMonthlyIDR = settingFloat(ctx, db, "verified_store_monthly_price", 15000)
	if s.IsVerified || s.IsAdmin {
		s.FreeBoostLimit = 3
	} else {
		s.FreeBoostLimit = 1
	}
	return s, true
}

// GetStore: toko milik user yang login (null kalau belum punya).
func GetStore(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		s, found := loadStore(r.Context(), db, uid)
		if !found {
			ok(w, map[string]any{"store": nil})
			return
		}
		ok(w, map[string]any{"store": s})
	}
}

// ListCategories: kategori untuk filter katalog. Publik, tidak ada data pribadi.
func ListCategories(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(),
			`SELECT c.id, c.name, c.slug, COALESCE(c.icon,''),
			        COUNT(p.id) FILTER (WHERE COALESCE(s.is_active, true))
			   FROM market_category c
			   LEFT JOIN market_product p ON p.category_id=c.id AND p.is_active AND p.deleted_at IS NULL
			   LEFT JOIN market_store s ON s.id=p.store_id
			  WHERE c.is_active
			  GROUP BY c.id, c.name, c.slug, c.icon ORDER BY c.name`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type cat struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Slug     string `json:"slug"`
			Icon     string `json:"icon,omitempty"`
			Products int    `json:"products"`
		}
		out := []cat{}
		for rows.Next() {
			var c cat
			if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Icon, &c.Products); err != nil {
				log.Printf("[WARN] scan ListCategories: %v", err)
				continue
			}
			out = append(out, c)
		}
		ok(w, map[string]any{"categories": out})
	}
}

// PublicStore: toko publik + produk aktifnya. Toko yang belum diaktifkan admin
// tetap bisa dilihat pemiliknya lewat /api/store (bukan lewat sini).
func PublicStore(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var s storeInfo
		err := db.QueryRow(r.Context(),
			`SELECT s.id, s.name, COALESCE(s.description,''), COALESCE(s.logo,''), s.is_active, (s.is_verified AND (s.verified_until IS NULL OR s.verified_until > now())), (u.role IN ('ADMIN','SUPER_ADMIN')), s.wallet_balance, s.created_at
				   FROM market_store s JOIN "user" u ON u.id=s.user_id WHERE s.id=$1`, id).
			Scan(&s.ID, &s.Name, &s.Desc, &s.Logo, &s.IsActive, &s.IsVerified, &s.IsAdmin, &s.Balance, &s.CreatedAt)
		if err != nil {
			fail(w, http.StatusNotFound, "toko tidak ditemukan")
			return
		}
		// Saldo dompet toko bukan informasi publik — nol-kan sebelum keluar.
		s.Balance = 0
		// rating & jumlah ulasan toko, plus jumlah produk/orders (publik, anonim ok).
		var rating *float64
		var rc int
		_ = db.QueryRow(r.Context(),
			`SELECT COALESCE(AVG(rating),0), COUNT(*) FROM market_review WHERE store_id=$1`, id).Scan(&rating, &rc)
		if rating != nil && *rating > 0 {
			s.Rating = *rating
			s.ReviewCount = rc
		}
		_ = db.QueryRow(r.Context(), `SELECT COUNT(*) FROM market_product WHERE store_id=$1 AND is_active AND deleted_at IS NULL`, id).Scan(&s.Products)
		_ = db.QueryRow(r.Context(), `SELECT COUNT(*) FROM market_order WHERE store_id=$1`, id).Scan(&s.Orders)
		rows, err := db.Query(r.Context(),
			`SELECT id, title, COALESCE(description,''), price, stock, is_demo, COALESCE(image_url,''), sales_count
			   FROM market_product WHERE store_id=$1 AND is_active AND deleted_at IS NULL ORDER BY pin_order NULLS LAST, sales_count DESC LIMIT 100`, id)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type p struct {
			ID         string  `json:"id"`
			Title      string  `json:"title"`
			Desc       string  `json:"description"`
			Price      float64 `json:"price"`
			Stock      int     `json:"stock"`
			IsDemo     bool    `json:"is_demo"`
			ImageURL   string  `json:"image_url,omitempty"`
			SalesCount int     `json:"sales_count"`
		}
		var products []p
		for rows.Next() {
			var x p
			if err := rows.Scan(&x.ID, &x.Title, &x.Desc, &x.Price, &x.Stock, &x.IsDemo, &x.ImageURL, &x.SalesCount); err != nil {
				log.Printf("[WARN] scan PublicStore products: %v", err)
				continue
			}
			products = append(products, x)
		}
		ok(w, map[string]any{"store": s, "products": products})
	}
}

// OpenStore: daftar/buka toko. Peran dinaikkan hanya kalau masih USER — ADMIN
// dan SUPER_ADMIN sengaja TIDAK ikut turun (versi lama menurunkannya).
func OpenStore(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Banner      string `json:"banner"`
			Logo        string `json:"logo"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		if len(in.Name) < 3 {
			fail(w, http.StatusBadRequest, "nama toko minimal 3 karakter")
			return
		}
		_, err := db.Exec(r.Context(),
			`INSERT INTO market_store (user_id, name, description, banner, logo, is_active)
			 VALUES ($1,$2,$3,$4,$5,true)
			 ON CONFLICT (user_id) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description,
			   banner=COALESCE(EXCLUDED.banner, market_store.banner),
			   logo=COALESCE(EXCLUDED.logo, market_store.logo), updated_at=now()`,
			uid, in.Name, nullStr(in.Description), nullStr(in.Banner), nullStr(in.Logo))
		if err != nil {
			log.Printf("[ERROR] buka toko gagal: %v", err)
			fail(w, http.StatusInternalServerError, "gagal membuka toko")
			return
		}
		if role := dbRole(r.Context(), db, uid); role == "USER" {
			if _, err := db.Exec(r.Context(),
				`UPDATE "user" SET role='SELLER', updated_at=now() WHERE id=$1 AND role='USER'`, uid); err != nil {
				log.Printf("[WARN] naikkan peran SELLER gagal: %v", err)
			}
		}
		audit(r.Context(), db, uid, "STORE_OPEN", uid, nil, map[string]any{"name": in.Name}, ipOf(r))
		s, _ := loadStore(r.Context(), db, uid)
		ok(w, map[string]any{"ok": true, "store": s})
	}
}

// StoreProducts: produk milik toko sendiri.
func StoreProducts(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		s, found := loadStore(r.Context(), db, uid)
		if !found {
			fail(w, http.StatusNotFound, "daftarkan toko dulu")
			return
		}
		rows, err := db.Query(r.Context(),
			`SELECT p.id, p.title, COALESCE(p.description,''), p.price, p.stock, p.is_instant, p.is_active, p.is_boosted,
			        p.sales_count, p.view_count, p.allow_affiliate, p.affiliate_percent, COALESCE(p.category_id::text,''), COALESCE(c.name,''), p.created_at,
			        p.boosted_until
			   FROM market_product p LEFT JOIN market_category c ON c.id = p.category_id
			  WHERE p.store_id=$1 AND p.deleted_at IS NULL ORDER BY p.created_at DESC`, s.ID)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type p struct {
			ID               string    `json:"id"`
			Title            string    `json:"title"`
			Description      string    `json:"description"`
			Price            float64   `json:"price"`
			Stock            int       `json:"stock"`
			IsInstant        bool      `json:"is_instant"`
			IsActive         bool      `json:"is_active"`
			IsBoosted        bool      `json:"is_boosted"`
			SalesCount       int       `json:"sales_count"`
			ViewCount        int       `json:"view_count"`
			AllowAffiliate   bool      `json:"allow_affiliate"`
			AffiliatePercent float64   `json:"affiliate_percent"`
			CategoryID       string    `json:"category_id,omitempty"`
			Category         string    `json:"category"`
			CreatedAt        time.Time `json:"created_at"`
			// Sisa masa boost produk: seller perlu tahu kapan boost-nya habis
			// supaya bisa memutuskan perpanjang atau tidak.
			BoostedUntil string `json:"boosted_until,omitempty"`
			// Nilai mentah nullable dari DB; tidak di-JSON-kan.
			BoostedUntilRaw *time.Time `json:"-"`
		}
		out := []p{}
		for rows.Next() {
			var x p
			if err := rows.Scan(&x.ID, &x.Title, &x.Description, &x.Price, &x.Stock, &x.IsInstant, &x.IsActive, &x.IsBoosted,
				&x.SalesCount, &x.ViewCount, &x.AllowAffiliate, &x.AffiliatePercent, &x.CategoryID, &x.Category, &x.CreatedAt, &x.BoostedUntilRaw); err != nil {
				continue
			}
			// Hanya kirim kalau boost masih berjalan; waktu lampau bikin UI
			// menampilkan "boost habis" padahal is_boosted sudah false.
			if x.BoostedUntilRaw != nil && x.BoostedUntilRaw.After(time.Now()) {
				x.BoostedUntil = x.BoostedUntilRaw.Format(time.RFC3339)
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in StoreProducts: %v", err)
		}
		ok(w, map[string]any{"store": s, "products": out})
	}
}

// Batas isian produk instan. Ketiganya ditegakkan di server; klien hanya
// menampilkan hitungannya.
const (
	minProductDescription = 20
	maxProductDescription = 2000
	maxInstantItems       = 10000
	maxInstantItemChars   = 4096
	maxInstantBytes       = 1 << 20
)

// productInput menampung field yang boleh ditulis seller. Query string kosong
// dipetakan ke NULL supaya kolom relasi tidak terisi "".
type productInput struct {
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Price            num      `json:"price"`
	Stock            num      `json:"stock"`
	CategoryID       string   `json:"category_id"`
	ImageURL         string   `json:"image_url"`
	Media            string   `json:"media"`
	IsDigital        *bool    `json:"is_digital"`
	IsInstant        *bool    `json:"is_instant"`
	InstantItems     []string `json:"instant_items"`
	AllowAffiliate   *bool    `json:"allow_affiliate"`
	AffiliatePercent num      `json:"affiliate_percent"`
	linkGuardText    string
}

func (in *productInput) validate() (float64, int, string) {
	title := strings.TrimSpace(in.Title)
	if len(title) < 3 {
		return 0, 0, "judul minimal 3 karakter"
	}
	if len(title) > 140 {
		return 0, 0, "judul terlalu panjang"
	}
	descriptionLength := utf8.RuneCountInString(strings.TrimSpace(in.Description))
	if descriptionLength < minProductDescription {
		return 0, 0, "deskripsi minimal 20 karakter"
	}
	if descriptionLength > maxProductDescription {
		return 0, 0, "deskripsi maksimal 2000 karakter"
	}
	price := in.Price.Float()
	if !(price >= 0) {
		return 0, 0, "harga tidak sah"
	}
	stock := in.Stock.Int(0)
	if stock < 0 {
		stock = 0
	}
	// Marketplace adalah tempat paling mudah menyelipkan tautan luar.
	if v := detectLinks(title + "\n" + in.Description); len(v) > 0 {
		return 0, 0, "link tidak diizinkan: " + strings.Join(v, " | ")
	}
	return price, stock, ""
}

func (in *productInput) affiliate() (bool, float64, string) {
	enabled := in.AllowAffiliate != nil && *in.AllowAffiliate
	percent := in.AffiliatePercent.Float()
	if !enabled {
		return false, 0, ""
	}
	if percent <= 0 || percent > 100 {
		return false, 0, "komisi afiliasi harus antara 1 sampai 100 persen"
	}
	return true, percent, ""
}

// instantItems: satu baris nonblank = satu barang yang dikirim sekali. Baris
// kosong dibuang, bukan ditolak — textarea/berkas TXT gampang menyisakan baris
// kosong di ujung. Panjang daftar inilah stok produk instan.
func (in *productInput) instantItems() ([]string, string) {
	if len(in.InstantItems) > maxInstantItems {
		return nil, "maksimal " + itoa(maxInstantItems) + " isian instan"
	}
	items := make([]string, 0, len(in.InstantItems))
	total := 0
	for _, raw := range in.InstantItems {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if len(s) > maxInstantItemChars {
			return nil, "isian instan maksimal " + itoa(maxInstantItemChars) + " karakter per baris"
		}
		total += len(s)
		if total > maxInstantBytes {
			return nil, "total isian instan melebihi 1 MB"
		}
		items = append(items, s)
	}
	if len(items) == 0 {
		return nil, "produk instan wajib punya minimal 1 isian"
	}
	return items, ""
}

// Isi barang instan kerap berupa kredensial (akun|password, kode aktivasi),
// jadi disimpan TERENKRIPSI AES-256-GCM lewat paket `secret` — pola yang sama
// dengan kunci provider & API key. Baris lama yang masih plaintext tetap
// terbaca: secret.Decrypt melewatkan nilai tanpa awalan "v1.".
//
// ponytail: satu kredensial per baris di market_product_item, bukan berkas di
// object storage. Batas 10.000 baris / 1 MB (maxInstantItems/maxInstantBytes)
// sudah cukup untuk kode digital; pindah ke object storage kalau seller perlu
// mengunggah berkas besar. Tidak ada log akses per-barang.
func encryptInstantItems(items []string) ([]string, error) {
	out := make([]string, 0, len(items))
	for _, s := range items {
		ct, err := secret.Encrypt(s)
		if err != nil {
			return nil, err
		}
		out = append(out, ct)
	}
	return out, nil
}

// decryptInstantItem: buka satu baris isian. Baris rusak (kunci berganti /
// data rusak) dikembalikan apa adanya supaya pengiriman tidak gagal total —
// polosnya terlihat di UI dan bisa dilaporkan, bukan diam-diam hilang.
func decryptInstantItem(stored string) string {
	plain, err := secret.Decrypt(stored)
	if err != nil {
		log.Printf("[WARN] buka isian instan gagal: %v", err)
		return stored
	}
	return plain
}

// CreateStoreProduct: POST /api/store/products.
//
// ponytail: produk baru langsung is_active (alur lama) supaya seller bisa
// berjualan tanpa menunggu admin; admin tetap bisa menariknya lewat
// PATCH /api/admin/products. Kalau moderasi wajib, balik jadi false di sini.
func CreateStoreProduct(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		s, found := loadStore(r.Context(), db, uid)
		if !found {
			fail(w, http.StatusNotFound, "daftarkan toko dulu")
			return
		}
		var in productInput
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		price, stock, msg := in.validate()
		if msg != "" {
			fail(w, http.StatusBadRequest, msg)
			return
		}
		digital := true
		if in.IsDigital != nil {
			digital = *in.IsDigital
		}
		affiliate, affiliatePercent, msg := in.affiliate()
		if msg != "" {
			fail(w, http.StatusBadRequest, msg)
			return
		}
		instant := in.IsInstant != nil && *in.IsInstant
		var items []string
		if instant {
			var msg string
			if items, msg = in.instantItems(); msg != "" {
				fail(w, http.StatusBadRequest, msg)
				return
			}
			// Stok produk instan = jumlah barangnya. Disamakan di server, bukan
			// dipercaya dari klien: selisih satu angka = barang terjual tanpa isi.
			stock = len(items)
		}

		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal membuat produk")
			return
		}
		defer tx.Rollback(r.Context())

		var id string
		err = tx.QueryRow(r.Context(),
			`INSERT INTO market_product
			   (seller_id, store_id, category_id, title, description, price, stock, image_url, media, is_digital, is_instant, allow_affiliate, affiliate_percent, is_active)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,true) RETURNING id`,
			uid, s.ID, nullStr(in.CategoryID), strings.TrimSpace(in.Title), nullStr(in.Description),
			price, stock, nullStr(in.ImageURL), nullStr(in.Media), digital, instant, affiliate, affiliatePercent).Scan(&id)
		if err != nil {
			log.Printf("[ERROR] buat produk gagal: %v", err)
			fail(w, http.StatusInternalServerError, "gagal membuat produk")
			return
		}
		// Produk + isinya satu transaksi: produk tanpa barang berarti pembeli
		// membayar sesuatu yang tidak bisa dikirim.
		if instant {
			enc, err := encryptInstantItems(items)
			if err != nil {
				log.Printf("[ERROR] enkripsi isian instan gagal: %v", err)
				fail(w, http.StatusInternalServerError, "gagal menyimpan isian instan")
				return
			}
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO market_product_item (product_id, content)
				 SELECT $1, unnest($2::text[])`, id, enc); err != nil {
				log.Printf("[ERROR] simpan isian instan gagal: %v", err)
				fail(w, http.StatusInternalServerError, "gagal menyimpan isian instan")
				return
			}
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "gagal membuat produk")
			return
		}
		ok(w, map[string]any{"ok": true, "id": id})
	}
}

// UpdateStoreProduct: PATCH /api/store/products/{id}. Hanya produk milik toko
// sendiri (syarat store_id ikut di dalam query).
func UpdateStoreProduct(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		s, found := loadStore(r.Context(), db, uid)
		if !found {
			fail(w, http.StatusNotFound, "daftarkan toko dulu")
			return
		}
		var in struct {
			productInput
			IsActive *bool `json:"is_active"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		if in.Title != "" {
			if _, _, msg := in.productInput.validate(); msg != "" {
				fail(w, http.StatusBadRequest, msg)
				return
			}
		}
		// Isian instan sengaja tidak bisa diganti lewat PATCH (isinya kredensial
		// dan ditulis sekali saat pembuatan). Diam-diam mengabaikannya membuat
		// seller mengira stoknya sudah berganti; tolak dengan alasan yang jelas.
		// Baris kosong diabaikan supaya form sunting yang mengirim [""] tidak
		// ditolak percuma.
		// ponytail: ganti = hapus produk + buat baru. Kalau perlu tambah/kurangi
		// isian tanpa membuang id produk, tambahkan endpoint isian terpisah.
		for _, s := range in.InstantItems {
			if strings.TrimSpace(s) != "" {
				fail(w, http.StatusBadRequest, "isian produk instan tidak bisa diubah lewat edit — hapus produk lalu buat ulang untuk menggantinya")
				return
			}
		}
		var affiliatePercent any
		if in.AllowAffiliate != nil {
			_, percent, msg := in.productInput.affiliate()
			if msg != "" {
				fail(w, http.StatusBadRequest, msg)
				return
			}
			affiliatePercent = percent
		}
		res, err := db.Exec(r.Context(),
			`UPDATE market_product SET
			   title            = COALESCE(NULLIF($1,''), title),
			   description      = COALESCE(NULLIF($2,''), description),
			   price            = COALESCE(NULLIF($3,-1), price),
			   stock            = COALESCE(NULLIF($4,-1), stock),
			   low_stock_notified = CASE WHEN COALESCE(NULLIF($4,-1), stock) > low_stock_threshold THEN false ELSE low_stock_notified END,
			   is_active        = COALESCE($5, is_active),
			   allow_affiliate  = COALESCE($6, allow_affiliate),
			   affiliate_percent = COALESCE($11, affiliate_percent),
			   category_id      = COALESCE(NULLIF($7,'')::uuid, category_id),
			   image_url        = COALESCE(NULLIF($10,''), image_url),
			   updated_at       = now()
			   WHERE id=$8 AND store_id=$9`,
			strings.TrimSpace(in.Title), in.Description, in.Price.Float(), in.Stock.Int(-1),
			in.IsActive, in.AllowAffiliate, in.CategoryID, r.PathValue("id"), s.ID,
			strings.TrimSpace(in.ImageURL), affiliatePercent)
		if err != nil {
			log.Printf("[ERROR] ubah produk gagal: %v", err)
			fail(w, http.StatusInternalServerError, "gagal mengubah produk")
			return
		}
		if res.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "produk tidak ditemukan")
			return
		}
		if in.AllowAffiliate != nil && !*in.AllowAffiliate {
			if _, err := db.Exec(r.Context(), `UPDATE affiliate_link SET is_active=false WHERE product_id=$1`, r.PathValue("id")); err != nil {
				log.Printf("[WARN] nonaktifkan link afiliasi gagal: %v", err)
			}
		}
		// is_instant hanya berubah kalau dikirim eksplisit: query di atas tidak
		// memuatnya supaya PATCH lama tidak diam-diam mengubah jenis produk.
		// ponytail: true → false menyisakan isian lama yang tidak lagi dipakai
		// (stok kembali manual); hapus isian kalau nanti perlu bersih-bersih.
		if in.IsInstant != nil {
			if _, err := db.Exec(r.Context(),
				`UPDATE market_product SET is_instant=$1, updated_at=now() WHERE id=$2 AND store_id=$3`,
				*in.IsInstant, r.PathValue("id"), s.ID); err != nil {
				log.Printf("[ERROR] ubah jenis instan gagal: %v", err)
				fail(w, http.StatusInternalServerError, "gagal mengubah produk")
				return
			}
		}
		ok(w, map[string]any{"ok": true})
	}
}

// SQL hapus/pulih lunak dipakai bersama handler dan uji: satu sumber, jadi uji
// menjalankan pernyataan yang benar-benar dieksekusi (bukan salinan).
const (
	productSoftDeleteSQL = `UPDATE market_product p SET deleted_at=now(), is_active=false, updated_at=now()
		  WHERE p.id=$1 AND p.store_id=$2 AND p.deleted_at IS NULL
		    AND NOT EXISTS (SELECT 1 FROM market_order o WHERE o.product_id=p.id)`
	productRestoreSQL = `UPDATE market_product SET deleted_at=NULL, is_active=true, updated_at=now()
		  WHERE id=$1 AND store_id=$2 AND deleted_at IS NOT NULL`
	productDeleteProbeSQL = `SELECT EXISTS(SELECT 1 FROM market_order o WHERE o.product_id=p.id),
		        p.deleted_at IS NOT NULL
		   FROM market_product p WHERE p.id=$1 AND p.store_id=$2`
)

// DeleteStoreProduct: DELETE /api/store/products/{id} — hapus LUNAK.
//
// Baris produk tidak dibuang: pesanan, keranjang, link afiliasi, boost, dan
// isian instan menunjuk produk_id dengan FK, jadi DELETE keras menghapus
// riwayat pembeli (CASCADE) atau membuat pesanan kehilangan judulnya. Yang
// terjadi sekarang: deleted_at diisi + is_active=false, sehingga seluruh
// query publik (yang sudah menyaring is_active=true) tidak lagi menampilkannya,
// dan POST …/restore membalikkannya (Undo).
//
// Produk yang sudah punya pesanan tetap 409 seperti sebelumnya — riwayat
// pesanan lebih baik tidak terganggu tombol hapus sama sekali.
func DeleteStoreProduct(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		s, found := loadStore(r.Context(), db, uid)
		if !found {
			fail(w, http.StatusNotFound, "daftarkan toko dulu")
			return
		}
		// Sudah dihapus = idempoten (klik Undo lalu hapus lagi tidak error).
		res, err := db.Exec(r.Context(), productSoftDeleteSQL, r.PathValue("id"), s.ID)
		if err != nil {
			log.Printf("[ERROR] hapus produk gagal: %v", err)
			fail(w, http.StatusInternalServerError, "gagal menghapus produk")
			return
		}
		if res.RowsAffected() == 0 {
			// Tidak terubah: pilih pesan yang benar (404 vs 409 vs sudah terhapus).
			var punyaOrder, sudahHapus bool
			if err := db.QueryRow(r.Context(), productDeleteProbeSQL,
				r.PathValue("id"), s.ID).Scan(&punyaOrder, &sudahHapus); err != nil {
				fail(w, http.StatusNotFound, "produk tidak ditemukan")
				return
			}
			if sudahHapus {
				ok(w, map[string]any{"ok": true})
				return
			}
			fail(w, http.StatusConflict, "produk sudah punya pesanan — nonaktifkan saja")
			return
		}
		audit(r.Context(), db, uid, "PRODUCT_DELETE", r.PathValue("id"),
			map[string]any{"deleted": false}, map[string]any{"deleted": true}, ipOf(r))
		ok(w, map[string]any{"ok": true})
	}
}

// RestoreStoreProduct: POST /api/store/products/{id}/restore — batalkan hapus.
// Hanya produk milik toko sendiri yang sedang terhapus; is_active dikembalikan
// ke true (ponytail: nilai sebelum hapus tidak dilacak — Undo = tayang lagi,
// admin tetap bisa menariknya lewat PATCH /api/admin/products).
func RestoreStoreProduct(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		s, found := loadStore(r.Context(), db, uid)
		if !found {
			fail(w, http.StatusNotFound, "daftarkan toko dulu")
			return
		}
		res, err := db.Exec(r.Context(), productRestoreSQL, r.PathValue("id"), s.ID)
		if err != nil {
			log.Printf("[ERROR] pulihkan produk gagal: %v", err)
			fail(w, http.StatusInternalServerError, "gagal memulihkan produk")
			return
		}
		if res.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "produk tidak ditemukan atau belum dihapus")
			return
		}
		audit(r.Context(), db, uid, "PRODUCT_RESTORE", r.PathValue("id"),
			map[string]any{"deleted": true}, map[string]any{"deleted": false}, ipOf(r))
		ok(w, map[string]any{"ok": true})
	}
}

// VerifyStore: POST /api/store/verify — langganan badge verified 30 hari.
func VerifyStore(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		price := settingFloat(r.Context(), db, "verified_store_monthly_price", 15000)
		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		defer tx.Rollback(r.Context())
		var storeID string
		if err := tx.QueryRow(r.Context(), `SELECT id FROM market_store WHERE user_id=$1 FOR UPDATE`, uid).Scan(&storeID); err != nil {
			fail(w, http.StatusNotFound, "toko belum tersedia")
			return
		}
		cut, err := tx.Exec(r.Context(), `UPDATE "user" SET wallet_balance=wallet_balance-$1 WHERE id=$2 AND wallet_balance >= $1`, price, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses pembayaran")
			return
		}
		if cut.RowsAffected() == 0 {
			fail(w, http.StatusPaymentRequired, "saldo tidak cukup")
			return
		}
		var balance float64
		if err := tx.QueryRow(r.Context(), `SELECT wallet_balance FROM "user" WHERE id=$1`, uid).Scan(&balance); err != nil {
			fail(w, http.StatusPaymentRequired, "saldo tidak cukup")
			return
		}
		if _, err := tx.Exec(r.Context(), `UPDATE market_store SET is_verified=true, verified_until=GREATEST(COALESCE(verified_until, now()), now()) + interval '30 days', updated_at=now() WHERE id=$1`, storeID); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mengaktifkan verified")
			return
		}
		// Catat langganan supaya reconciler bisa memperpanjang otomatis.
		// auto_renew=false di awal: user harus memilih sendiri (opt-in), bukan
		// diam-diam menagih tiap bulan. Harga periode berikutnya belum dikunci
		// sampai user menyalakan auto-renew.
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO subscription (user_id, tier_name, price, status, current_period_start, current_period_end, auto_renew, renew_days)
			 VALUES ($1,$2,$3,'ACTIVE',now(),now()+interval '30 days',false,30)
			 ON CONFLICT (user_id, tier_name) WHERE status='ACTIVE'
			 DO UPDATE SET status='ACTIVE', price=$3,
			               current_period_end=GREATEST(subscription.current_period_end, now())+interval '30 days',
			               last_error='', updated_at=now()`,
			uid, tierVerifiedStore, price); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mencatat langganan")
			return
		}
		if _, err := tx.Exec(r.Context(), `INSERT INTO credit_transaction (user_id, amount, type, status, description) VALUES ($1,$2,'VERIFY_STORE','COMPLETED','Verified Store 30 hari')`, uid, -price); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mencatat pembayaran")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan")
			return
		}
		ok(w, map[string]any{"ok": true, "price": price, "days": 30})
	}
}

// BuyBoostPack: membeli kredit boost dari dompet user, bukan saldo toko.
func BuyBoostPack(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		count, price := 0, 0.0
		switch r.PathValue("count") {
		case "5":
			count, price = 5, 5000
		case "15":
			count, price = 15, 15000
		case "50":
			count, price = 50, 30000
		default:
			fail(w, http.StatusBadRequest, "paket boost tidak valid")
			return
		}
		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, 500, "gagal memproses")
			return
		}
		defer tx.Rollback(r.Context())
		var storeID string
		if err := tx.QueryRow(r.Context(), `SELECT id FROM market_store WHERE user_id=$1 FOR UPDATE`, uid).Scan(&storeID); err != nil {
			fail(w, 404, "toko belum tersedia")
			return
		}
		cut, err := tx.Exec(r.Context(), `UPDATE "user" SET wallet_balance=wallet_balance-$1 WHERE id=$2 AND wallet_balance >= $1`, price, uid)
		if err != nil || cut.RowsAffected() == 0 {
			fail(w, http.StatusPaymentRequired, "saldo wallet tidak cukup")
			return
		}
		if _, err := tx.Exec(r.Context(), `UPDATE market_store SET boost_credits=boost_credits+$1, updated_at=now() WHERE id=$2`, count, storeID); err != nil {
			fail(w, 500, "gagal menambah kredit boost")
			return
		}
		var id string
		if err := tx.QueryRow(r.Context(), `INSERT INTO credit_transaction (user_id,amount,type,status,description) VALUES ($1,$2,'BOOST_PURCHASE','COMPLETED',$3) RETURNING id`, uid, -price, fmt.Sprintf("Paket Boost x%d", count)).Scan(&id); err != nil {
			fail(w, 500, "gagal mencatat pembelian")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, 500, "gagal menyimpan")
			return
		}
		ok(w, map[string]any{"ok": true, "id": id, "credits": count, "price": price})
	}
}

// BoostProduct: POST /api/store/products/{id}/boost. Satu boost berlaku 6 jam.
func BoostProduct(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		s, found := loadStore(r.Context(), db, uid)
		if !found {
			fail(w, http.StatusNotFound, "daftarkan toko dulu")
			return
		}
		var in struct {
			Count int `json:"count"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		if in.Count == 0 {
			in.Count = 1
		}
		if !boostCountAllowed(in.Count) {
			fail(w, http.StatusBadRequest, "jumlah boost harus 1 sampai 50")
			return
		}
		remaining, err := activateBoost(r.Context(), db, uid, r.PathValue("id"), s.ID, in.Count)
		if errors.Is(err, errBoostCredits) {
			fail(w, http.StatusPaymentRequired, "sisa boost tidak cukup; beli paket boost terlebih dahulu")
			return
		}
		if err != nil {
			fail(w, http.StatusBadRequest, "gagal boost: "+err.Error())
			return
		}
		ok(w, map[string]any{"ok": true, "count": in.Count, "hours": boostHours(in.Count), "credits": remaining})
	}
}

var errBoostCredits = errors.New("boost credits tidak cukup")

func boostCountAllowed(count int) bool { return count >= 1 && count <= 50 }
func boostHours(count int) int         { return count * 6 }

// Kredit, audit, dan durasi berubah dalam satu transaksi. Kondisi pada UPDATE
// adalah pagar atomik agar dua request bersamaan tidak membuat saldo minus.
func activateBoost(ctx context.Context, db DB, uid, productID, storeID string, count int) (int, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var remaining int
	err = tx.QueryRow(ctx,
		`UPDATE market_store SET boost_credits=boost_credits-$1, updated_at=now()
		 WHERE id=$2 AND boost_credits >= $1 RETURNING boost_credits`, count, storeID).Scan(&remaining)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, errBoostCredits
	}
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO market_boost (store_id,user_id,product_id,days,cost,expires_at,paid_at,source)
		 VALUES ($1,$2,$3,$4,0,now()+make_interval(hours=>$5),now(),'CREDIT')`, storeID, uid, productID, count, boostHours(count))
	if err != nil {
		return 0, err
	}

	// Mark product as boosted. Produk yang sudah dihapus (hapus lunak) atau
	// ditarik admin tidak boleh ditagih: saldo toko sudah terpotong di atas,
	// jadi UPDATE 0 baris = batalkan seluruh transaksi (Rollback di defer).
	res, err := tx.Exec(ctx,
		`UPDATE market_product SET is_boosted=true,
		        boosted_until=GREATEST(COALESCE(boosted_until,now()),now())+make_interval(hours=>$3)
		 WHERE id=$1 AND store_id=$2 AND is_active AND deleted_at IS NULL`, productID, storeID, boostHours(count))
	if err != nil {
		return 0, err
	}
	if res.RowsAffected() == 0 {
		return 0, errors.New("produk tidak aktif atau sudah dihapus")
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return remaining, nil
}

// ------------------------------------------------------------------ ULASAN

func ListReviews(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		storeID := r.URL.Query().Get("store_id")
		productID := r.URL.Query().Get("product_id")
		rows, err := db.Query(r.Context(),
			`SELECT id, store_id, COALESCE(product_id::text,''), COALESCE(buyer_name,''), rating, COALESCE(comment,''), created_at
			   FROM market_review
			  WHERE ($1 = '' OR store_id::text = $1) AND ($2 = '' OR product_id::text = $2)
			  ORDER BY created_at DESC LIMIT 100`, storeID, productID)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type rv struct {
			ID        string    `json:"id"`
			StoreID   string    `json:"store_id"`
			ProductID string    `json:"product_id,omitempty"`
			BuyerName string    `json:"buyer_name"`
			Rating    int       `json:"rating"`
			Comment   string    `json:"comment"`
			CreatedAt time.Time `json:"created_at"`
		}
		out := []rv{}
		sum, n := 0, 0
		for rows.Next() {
			var x rv
			if err := rows.Scan(&x.ID, &x.StoreID, &x.ProductID, &x.BuyerName, &x.Rating, &x.Comment, &x.CreatedAt); err != nil {
				continue
			}
			out = append(out, x)
			sum += x.Rating
			n++
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in ListReviews: %v", err)
		}
		avg := 0.0
		if n > 0 {
			avg = float64(sum) / float64(n)
		}
		ok(w, map[string]any{"reviews": out, "count": n, "average": avg})
	}
}

// CreateReview: POST /api/market/reviews. Ulasan produk hanya boleh dari yang
// benar-benar membelinya — ulasan palsu dari mana saja membuat rating tak berarti.
func CreateReview(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			StoreID   string `json:"store_id"`
			ProductID string `json:"product_id,omitempty"`
			Rating    num    `json:"rating"`
			Comment   string `json:"comment"`
		}
		if err := decode(r, &in); err != nil || in.StoreID == "" {
			fail(w, http.StatusBadRequest, "store_id wajib")
			return
		}
		rating := in.Rating.Int(0)
		if rating < 1 || rating > 5 {
			fail(w, http.StatusBadRequest, "rating 1–5")
			return
		}
		// Ulasan hanya boleh dari pembeli, dan produk yang diulas harus benar
		// milik toko yang diulas — kalau tidak, pembeli produk murah di toko A
		// bisa menulis ulasan bintang 1 di toko B.
		if in.ProductID != "" {
			var bought bool
			if err := db.QueryRow(r.Context(),
				`SELECT true FROM market_order WHERE buyer_id=$1 AND product_id=$2 AND status IN ('PAID','PROCESSING','COMPLETED') LIMIT 1`,
				uid, in.ProductID).Scan(&bought); err != nil {
				fail(w, http.StatusForbidden, "ulasan hanya untuk produk yang sudah dibeli")
				return
			}
			var belongs bool
			if err := db.QueryRow(r.Context(),
				`SELECT true FROM market_product WHERE id=$1 AND store_id=$2`, in.ProductID, in.StoreID).Scan(&belongs); err != nil {
				fail(w, http.StatusBadRequest, "produk bukan milik toko ini")
				return
			}
		} else {
			// Ulasan tingkat toko: tetap wajib pernah bertransaksi di toko itu.
			var bought bool
			if err := db.QueryRow(r.Context(),
				`SELECT true FROM market_order WHERE buyer_id=$1 AND store_id=$2 AND status IN ('PAID','PROCESSING','COMPLETED') LIMIT 1`,
				uid, in.StoreID).Scan(&bought); err != nil {
				fail(w, http.StatusForbidden, "ulasan hanya untuk toko yang pernah Anda beli")
				return
			}
		}
		var name string
		_ = db.QueryRow(r.Context(), `SELECT COALESCE(name, email) FROM "user" WHERE id=$1`, uid).Scan(&name)

		var id string
		err := db.QueryRow(r.Context(),
			`INSERT INTO market_review (store_id, product_id, buyer_id, buyer_name, rating, comment)
			 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
			in.StoreID, nullStr(in.ProductID), uid, nullStr(name), rating, nullStr(in.Comment)).Scan(&id)
		if err != nil {
			log.Printf("[ERROR] insert ulasan gagal: %v", err)
			fail(w, http.StatusBadRequest, "toko atau produk tidak ditemukan")
			return
		}
		ok(w, map[string]any{"ok": true, "id": id})
	}
}

// ------------------------------------------------------------------ MEDIA

var mediaTypes = map[string]string{
	"image/png": "png", "image/jpeg": "jpg", "image/webp": "webp", "image/gif": "gif",
}

const maxMediaBytes = 1 << 20

// ponytail: cwebp menangani PNG/JPEG/WebP/GIF; GIF animasi jadi frame pertama.
// Butuh `libwebp-tools` di image runtime (lihat Dockerfile).
func convertToWebP(data []byte, lossless bool) ([]byte, error) {
	in, err := os.CreateTemp("", "node2mint-media-*")
	if err != nil {
		return nil, err
	}
	inName := in.Name()
	outName := inName + ".webp"
	defer os.Remove(inName)
	defer os.Remove(outName)
	if _, err = in.Write(data); err == nil {
		err = in.Close()
	} else {
		_ = in.Close()
	}
	if err != nil {
		return nil, err
	}
	args := []string{"-quiet", "-q", "82"}
	if lossless {
		args = []string{"-quiet", "-lossless", "-z", "9"}
	}
	args = append(args, inName, "-o", outName)
	if err = exec.Command("cwebp", args...).Run(); err != nil {
		return nil, err
	}
	webp, err := os.ReadFile(outName)
	if err != nil {
		return nil, err
	}
	if len(webp) == 0 || len(webp) > maxMediaBytes {
		return nil, fmt.Errorf("hasil WebP di luar batas ukuran (%d byte)", len(webp))
	}
	return webp, nil
}

// Kuota media per akun. Angka ini pilihan produk (bukan hasil pengukuran):
// cukup untuk katalog toko normal, dan tetap membatasi satu akun iseng.
const (
	maxMediaFiles      = 200
	maxMediaTotalBytes = 100 << 20
)

// MediaQuotaExceeded: true bila unggahan berikutnya melampaui kuota akun.
// Dipisah dari UploadMedia supaya batasnya bisa diuji tanpa mengunggah 200 berkas.
func MediaQuotaExceeded(files, used int64, nextBytes int) bool {
	return files >= maxMediaFiles || used+int64(nextBytes) > maxMediaTotalBytes
}

// UploadMedia: POST /api/media (multipart, field "file") → media_asset.data.
func UploadMedia(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		r.Body = http.MaxBytesReader(w, r.Body, maxMediaBytes+1024)
		if err := r.ParseMultipartForm(maxMediaBytes); err != nil {
			fail(w, http.StatusRequestEntityTooLarge, "berkas terlalu besar (maksimal 1 MB)")
			return
		}
		file, hdr, err := r.FormFile("file")
		if err != nil {
			fail(w, http.StatusBadRequest, "tidak ada berkas yang dikirim")
			return
		}
		defer file.Close()

		// Jenis ditentukan dari ISI berkas, bukan dari nama kiriman: nama
		// kiriman bisa berisi apa saja. 512 byte dibaca karena itu yang
		// dipakai http.DetectContentType untuk mengenali PNG/JPEG/GIF; buffer
		// ini ikut disimpan supaya berkas tidak cacat (byte yang dibaca untuk
		// deteksi tidak boleh hilang).
		head := make([]byte, 512)
		n, _ := io.ReadFull(file, head)
		head = head[:n]
		mime := http.DetectContentType(head)
		if _, allowed := mediaTypes[mime]; !allowed {
			fail(w, http.StatusUnsupportedMediaType, "jenis berkas harus PNG, JPEG, WebP, atau GIF")
			return
		}
		rest, err := io.ReadAll(io.LimitReader(file, maxMediaBytes))
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal membaca berkas")
			return
		}
		data := append(head, rest...)
		if len(data) > maxMediaBytes {
			fail(w, http.StatusRequestEntityTooLarge, "berkas terlalu besar (maksimal 1 MB)")
			return
		}
		data, err = convertToWebP(data, false)
		if err != nil {
			log.Printf("[ERROR] konversi WebP gagal: %v", err)
			fail(w, http.StatusInternalServerError, "gagal mengonversi gambar ke WebP")
			return
		}
		mime = "image/webp"
		fileName := strings.TrimSuffix(hdr.Filename, filepath.Ext(hdr.Filename)) + ".webp"
		// Kuota per akun: tanpa ini satu akun bisa mengisi penuh volume Postgres
		// (data disimpan sebagai bytea) dan menghentikan penulisan untuk semua.
		var used, files int64
		if err := db.QueryRow(r.Context(),
			`SELECT COALESCE(SUM(bytes),0), COUNT(*) FROM media_asset WHERE user_id=$1`, uid).Scan(&used, &files); err != nil {
			fail(w, http.StatusInternalServerError, "gagal memeriksa kuota media")
			return
		}
		if MediaQuotaExceeded(files, used, len(data)) {
			fail(w, http.StatusRequestEntityTooLarge, "kuota media habis (200 berkas / 100 MB per akun)")
			return
		}

		var id string
		err = db.QueryRow(r.Context(),
			`INSERT INTO media_asset (user_id, file_name, mime_type, bytes, data) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			uid, fileName, mime, len(data), data).Scan(&id)
		if err != nil {
			log.Printf("[ERROR] simpan media gagal: %v", err)
			fail(w, http.StatusInternalServerError, "gagal menyimpan berkas")
			return
		}
		ok(w, map[string]any{"ok": true, "id": id, "url": "/api/media/" + id, "mime_type": mime, "bytes": len(data)})
	}
}

// ServeMedia menyajikan aset media. Aset privat (belum dipakai di permukaan
// publik) hanya terlihat pemiliknya; anonim dan bukan pemilik sama-sama 404.
//
// Pengecualian penting: gambar produk & logo/banner toko MENUNJUK ke
// /api/media/<id>, jadi kalau syaratnya hanya user_id, setiap pengunjung
// (dan pembeli) melihat gambar rusak — hanya pengunggah yang bisa melihat.
// Satu query yang sama menyajikan berkas itu ketika dirujuk baris yang memang
// publik (produk aktif, atau toko mana pun yang punya halaman /api/store/{id}).
func ServeMedia(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, role := "", ""
		if raw := auth.RawToken(r); raw != "" {
			_ = db.QueryRow(r.Context(),
				`SELECT s.user_id, u.role FROM user_session s JOIN "user" u ON u.id=s.user_id
				 WHERE s.session_token=$1 AND s.expires>now() AND NOT u.is_banned`,
				provider.HashKey(raw)).Scan(&uid, &role)
		}
		// uid kosong dikirim sebagai NULL: user_id bertipe uuid, dan '' sebagai
		// uuid melempar error cast (seluruh query gagal → semua 404, termasuk
		// gambar yang seharusnya publik).
		var mime, name string
		var data []byte
		err := db.QueryRow(r.Context(),
			`SELECT m.mime_type, m.file_name, COALESCE(m.data, ''::bytea)
			   FROM media_asset m
			  WHERE m.id=$1
			    AND (m.user_id = NULLIF($2,'')::uuid
			         OR $3
			         OR EXISTS (SELECT 1 FROM market_product p
			                     JOIN market_store ps ON ps.id = p.store_id
			                    WHERE p.is_active AND ps.is_active
			                      -- pemilik berkas harus = pemilik toko, bukan sekadar
			                      -- "ada yang merujuknya": kalau tidak, seller bisa
			                      -- memublikasikan berkas milik orang lain lewat image_url.
			                      AND m.user_id = ps.user_id
			                      AND (p.image_url = '/api/media/' || m.id::text
			                           OR p.media LIKE '%/api/media/' || m.id::text || '%'))
			         OR EXISTS (SELECT 1 FROM market_store s
			                    WHERE s.user_id = m.user_id
			                      AND (s.logo = '/api/media/' || m.id::text
			                           OR s.banner = '/api/media/' || m.id::text))
			         OR EXISTS (SELECT 1 FROM platform_setting ps
			                    WHERE ps.key='market_banners'
			                      AND ps.value LIKE '%/api/media/' || m.id::text || '%'))`,
			r.PathValue("id"), uid, isAdminRole(role)).Scan(&mime, &name, &data)
		if err != nil {
			fail(w, http.StatusNotFound, "tidak ditemukan")
			return
		}
		// Aset lama (diunggah sebelum jalur unggah memaksa WebP) dikonversi di
		// sini sekali lalu ditulis balik ke barisnya, supaya setiap gambar yang
		// tampil di situs benar-benar WebP tanpa skrip migrasi terpisah.
		// ponytail: konversi terjadi di jalur permintaan (sekitar 50 ms sekali
		// per aset), bukan backfill offline; pindahkan ke job terjadwal kalau
		// jumlah aset lama sudah besar.
		if mime != "image/webp" && mediaTypes[mime] != "" {
			if converted, cerr := convertToWebP(data, false); cerr == nil {
				name = strings.TrimSuffix(name, filepath.Ext(name)) + ".webp"
				mime = "image/webp"
				data = converted
				if _, uerr := db.Exec(r.Context(),
					`UPDATE media_asset SET data=$2, mime_type='image/webp', file_name=$3, bytes=$4 WHERE id=$1`,
					r.PathValue("id"), converted, name, len(converted)); uerr != nil {
					log.Printf("[WARN] simpan WebP hasil konversi gagal: %v", uerr)
				}
			} else {
				log.Printf("[WARN] konversi WebP aset lama gagal: %v", cerr)
			}
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Content-Disposition", "inline; filename="+strconv.Quote(name))
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = w.Write(data)
	}
}

// ------------------------------------------------------------------ PUBLIK

// PublicModels: GET /api/models — katalog + harga + badge. Tidak memuat kunci,
// URL internal, atau pesan error provider.
func PublicModels(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(),
			`SELECT m.id, m.name, m.type, COALESCE(m.description,''), COALESCE(b.type,''), COALESCE(b.color,''),
			        COALESCE(m.context_length,0), m.tier, m.token_multiplier, COALESCE(m.family,''), health.last_checked
			   FROM ai_model m
			   LEFT JOIN model_badge b ON b.model_id = m.id
			   LEFT JOIN LATERAL (
			     SELECT h.checked_at AS last_checked
			       FROM model_health_log h
			      WHERE h.model_id=m.id
			      ORDER BY h.checked_at DESC LIMIT 1
			   ) health ON true
			  WHERE m.is_active AND (
			    EXISTS (
			      SELECT 1 FROM model_provider_route r
			      JOIN provider p ON p.id=r.provider_id
			       WHERE r.model_id=m.id AND r.is_active AND p.is_active
			         AND (btrim(COALESCE(p.models,''))='' OR EXISTS (
				       SELECT 1 FROM unnest(string_to_array(p.models,',')) AS candidate
				        WHERE btrim(candidate)=r.upstream_model
				     ))
			    ) OR EXISTS (
			      SELECT 1 FROM provider p
			       WHERE p.is_active AND (btrim(COALESCE(p.models,''))='' OR EXISTS (
			         SELECT 1 FROM unnest(string_to_array(p.models,',')) AS candidate
			          WHERE btrim(candidate)=m.name
			       ))
			  ))
			  ORDER BY `+familyOrderSQL+`, m.sort_weight DESC, m.name`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type m struct {
			ID          string     `json:"id"`
			Name        string     `json:"name"`
			Type        string     `json:"type"`
			Description string     `json:"description"`
			Badge       string     `json:"badge,omitempty"`
			BadgeColor  string     `json:"badge_color,omitempty"`
			ContextLen  int        `json:"context_length,omitempty"`
			Tier        string     `json:"tier"`
			Mult        float64    `json:"token_multiplier"`
			Family      string     `json:"family,omitempty"`
			LastChecked *time.Time `json:"last_checked,omitempty"`
		}
		out := []m{}
		for rows.Next() {
			var x m
			if err := rows.Scan(&x.ID, &x.Name, &x.Type, &x.Description, &x.Badge, &x.BadgeColor, &x.ContextLen, &x.Tier, &x.Mult, &x.Family, &x.LastChecked); err == nil {
				out = append(out, x)
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in PublicModels: %v", err)
		}
		var freeDailyTokens int64
		_ = db.QueryRow(r.Context(), `SELECT COALESCE(value::bigint, 0) FROM app_setting WHERE key='free_daily_tokens'`).Scan(&freeDailyTokens)
		ok(w, map[string]any{
			"models":       out,
			"gateway_live": GatewayLive(r.Context(), db),
			"billing": map[string]any{
				"unit":                  "api_token",
				"free_daily_tokens":     freeDailyTokens,
				"cached_token_discount": settingFloat(r.Context(), db, "cached_token_discount", 0.5),
				"usage_surcharge_rate":  settingFloat(r.Context(), db, "usage_surcharge_rate", 0.25),
			},
		})
	}
}

// PublicStatus: GET /api/status — uptime terakhir per monitor + ringkasan per
// MODEL dari model_health_log (availability 24 jam, latensi rata-rata & p90).
// Hanya nama, status, dan angka: URL internal, status code, dan pesan error
// tidak ikut. Trafik nyata pengguna tidak dihitung (usage_log tanpa latensi).
func PublicStatus(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Publik cuma butuh status keseluruhan (overall) + health model —
		// nama monitor, latency, dan uptime % tidak boleh bocor ke anonim.
		// Status "OPERATIONAL" hanya boleh keluar kalau setidaknya satu probe
		// benar-benar tercatat dalam 24 jam terakhir.
		var last string
		var monitorChecks, monitorUp int
		err := db.QueryRow(r.Context(), `SELECT
			COALESCE((SELECT h.status FROM uptime_heartbeat h
			  JOIN uptime_monitor m ON m.id=h.monitor_id AND m.is_active
			 WHERE h.checked_at > now() - interval '24 hours'
			 ORDER BY h.checked_at DESC LIMIT 1),'unknown'),
			(SELECT COUNT(*) FROM uptime_heartbeat h
			  JOIN uptime_monitor m ON m.id=h.monitor_id AND m.is_active
			 WHERE h.checked_at > now() - interval '24 hours'),
			(SELECT COUNT(*) FROM uptime_heartbeat h
			  JOIN uptime_monitor m ON m.id=h.monitor_id AND m.is_active
			 WHERE h.checked_at > now() - interval '24 hours' AND h.status <> 'down')`).Scan(&last, &monitorChecks, &monitorUp)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		models, mAllUp := modelHealth(r.Context(), db)
		modelChecks := 0
		for _, model := range models {
			modelChecks += model.Checks
		}
		overall := publicStatusOverall(last, monitorChecks, modelChecks, mAllUp)
		// Daftar monitor (nama internal, latency, uptime) TIDAK dipublikasikan —
		// cuma ringkasan overall + health model. Detail lengkap khusus admin lewat
		// AdminMonitors (/api/admin/monitors).
		ok(w, map[string]any{
			"models":    models,
			"site_name": settingStr(r.Context(), db, "site_name", "Node2Mint"), "overall": overall,
			"availability_24h": func() float64 {
				if monitorChecks == 0 {
					return 0
				}
				return float64(monitorUp) / float64(monitorChecks) * 100
			}(),
			"gateway_live": GatewayLive(r.Context(), db),
		})
	}
}

func publicStatusOverall(monitorStatus string, monitorChecks, modelChecks int, modelsHealthy bool) string {
	if monitorChecks+modelChecks == 0 {
		return "UNKNOWN"
	}
	if monitorStatus == "down" || !modelsHealthy {
		return "DEGRADED"
	}
	return "OPERATIONAL"
}

// AdminMonitors: GET /api/admin/monitors — daftar monitor lengkap untuk halaman
// status versi admin.
func AdminMonitors(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(),
			`SELECT m.name, m.kind,
			        COALESCE((SELECT h.status FROM uptime_heartbeat h WHERE h.monitor_id=m.id ORDER BY h.checked_at DESC LIMIT 1),'unknown'),
			        (SELECT h.latency_ms FROM uptime_heartbeat h WHERE h.monitor_id=m.id ORDER BY h.checked_at DESC LIMIT 1),
			        (SELECT h.checked_at FROM uptime_heartbeat h WHERE h.monitor_id=m.id ORDER BY h.checked_at DESC LIMIT 1),
			        (SELECT COUNT(*) FROM uptime_heartbeat h WHERE h.monitor_id=m.id AND h.checked_at > now() - interval '24 hours'),
			        (SELECT COUNT(*) FROM uptime_heartbeat h WHERE h.monitor_id=m.id AND h.checked_at > now() - interval '24 hours' AND h.status <> 'down')
			   FROM uptime_monitor m WHERE m.is_active ORDER BY m.name`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type mon struct {
			Name      string     `json:"name"`
			Kind      string     `json:"kind"`
			Status    string     `json:"status"`
			LatencyMs *int       `json:"latency_ms,omitempty"`
			CheckedAt *time.Time `json:"checked_at,omitempty"`
			UptimePct *float64   `json:"uptime_pct,omitempty"`
		}
		out := []mon{}
		for rows.Next() {
			var x mon
			var beats, up *int
			var latency *int
			var checked *time.Time
			if err := rows.Scan(&x.Name, &x.Kind, &x.Status, &latency, &checked, &beats, &up); err != nil {
				continue
			}
			x.LatencyMs = latency
			x.CheckedAt = checked
			if beats != nil && *beats > 0 && up != nil {
				pct := float64(*up) / float64(*beats) * 100
				x.UptimePct = &pct
			}
			out = append(out, x)
		}
		ok(w, map[string]any{"monitors": out})
	}
}

type modelHealthRow struct {
	Name         string     `json:"name"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	Availability *float64   `json:"availability,omitempty"` // % probe 24 jam yang tidak down
	AvgLatencyMs *int       `json:"avg_latency_ms,omitempty"`
	P90LatencyMs *int       `json:"p90_latency_ms,omitempty"`
	Checks       int        `json:"checks"`
	LastChecked  *time.Time `json:"last_checked,omitempty"`
	Spark        []string   `json:"spark"` // 24 ember jam: ok|slow|down|error|empty
}

// modelHealth: satu query — model katalog (kiri) + agregat health log 24 jam.
// Spark = pasangan "YYYY-MM-DD-HH|status" hasil agregat per jam (status terburuk
// di jam itu menang); Go memetakannya ke 24 ember.
func modelHealth(ctx context.Context, db DB) ([]modelHealthRow, bool) {
	rows, err := db.Query(ctx,
		`SELECT m.name, m."type",
		        COALESCE(l.status,'no_traffic'), l.cnt, l.avg_ms, l.p90_ms, l.last_at, l.up, l.spark
		   FROM ai_model m
		   LEFT JOIN LATERAL (
		     SELECT (SELECT h.status FROM model_health_log h WHERE h.model_id=m.id
		              ORDER BY h.checked_at DESC LIMIT 1) AS status,
		            COUNT(*) AS cnt,
		            AVG(h.latency_ms)::int AS avg_ms,
		            percentile_disc(0.9) WITHIN GROUP (ORDER BY h.latency_ms)::int AS p90_ms,
		            MAX(h.checked_at) AS last_at,
		            COUNT(*) FILTER (WHERE h.status <> 'down') AS up,
		            (SELECT array_agg(to_char(z.hour,'YYYY-MM-DD-HH') || '|' || z.st ORDER BY z.hour)
		               FROM (SELECT date_trunc('hour', h2.checked_at) AS hour,
		                            CASE WHEN bool_or(h2.status='down')  THEN 'down'
		                                 WHEN bool_or(h2.status='error') THEN 'error'
		                                 WHEN bool_or(h2.status='slow')  THEN 'slow'
		                                 ELSE 'ok' END AS st
		                       FROM model_health_log h2
		                      WHERE h2.model_id = m.id AND h2.checked_at > now() - interval '24 hours'
		                      GROUP BY 1) z) AS spark
		       FROM model_health_log h
		      WHERE h.model_id = m.id AND h.checked_at > now() - interval '24 hours'
		   ) l ON true
		  WHERE m.is_active AND NOT (string_to_array(m."type", ',') && ARRAY['EMBEDDING','RERANK'])
		  ORDER BY m.sort_weight DESC, m.name`)
	if err != nil {
		log.Printf("[WARN] query error in modelHealth: %v", err)
		return nil, true
	}
	defer rows.Close()

	out := []modelHealthRow{}
	allUp := true
	for rows.Next() {
		var x modelHealthRow
		var status string
		var cnt, avg, p90 *int
		var lastAt *time.Time
		var up *int
		var sparkArr []string
		if err := rows.Scan(&x.Name, &x.Type, &status, &cnt, &avg, &p90, &lastAt, &up, &sparkArr); err != nil {
			continue
		}
		x.Status = status
		x.Spark = sparkBuckets(sparkArr)
		if cnt != nil {
			x.Checks = *cnt
		}
		x.AvgLatencyMs = avg
		x.P90LatencyMs = p90
		x.LastChecked = lastAt
		// Availability hanya bermakna kalau ada probe; tanpa probe jangan 0%.
		if cnt != nil && *cnt > 0 && up != nil {
			pct := float64(*up) / float64(*cnt) * 100
			x.Availability = &pct
			if pct < 99 {
				allUp = false
			}
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[WARN] rows error in modelHealth: %v", err)
	}
	return out, allUp
}

// sparkBuckets: pasangan "YYYY-MM-DD-HH|status" → 24 ember jam terakhir
// (indeks terakhir = jam sekarang). Ember tanpa probe = "" (digambar abu-abu).
func sparkBuckets(pairs []string) []string {
	const n = 24
	out := make([]string, n)
	nowHour := time.Now().UTC().Truncate(time.Hour)
	for _, p := range pairs {
		hourStr, st, ok := strings.Cut(p, "|")
		if !ok {
			continue
		}
		hour, err := time.ParseInLocation("2006-01-02-15", hourStr, time.UTC)
		if err != nil {
			continue
		}
		idx := n - 1 - int(nowHour.Sub(hour)/time.Hour)
		if idx < 0 || idx >= n {
			continue
		}
		out[idx] = st
	}
	return out
}

// ------------------------------------------------------------------ ARTIKEL

var nonAlnum = regexp.MustCompile(`[^a-z0-9\s-]`)
var spaces = regexp.MustCompile(`\s+`)

func slugify(title string) string {
	s := spaces.ReplaceAllString(nonAlnum.ReplaceAllString(strings.ToLower(title), ""), "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
	}
	if s == "" {
		s = "artikel"
	}
	return s
}

func insertArticle(ctx context.Context, db DB, authorID, role string, admin bool, title, content, excerpt, image string, published bool) (string, string, error) {
	content = sanitizeArticleHTML(content)
	base := slugify(title)
	slug := base
	for i := 2; i <= 50; i++ {
		var taken bool
		if err := db.QueryRow(ctx, `SELECT true FROM article WHERE slug=$1`, slug).Scan(&taken); err != nil {
			break
		}
		slug = base + "-" + itoa(i)
	}
	var publishedAt any
	if published {
		publishedAt = time.Now()
	}
	status := "PENDING"
	if published {
		status = "PUBLISHED"
	}
	var id string
	err := db.QueryRow(ctx,
		`INSERT INTO article (author_id, author_role, title, slug, content, excerpt, image, status, is_admin_post, published_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		authorID, role, title, slug, content, nullStr(excerpt), nullStr(image), status, admin, publishedAt).Scan(&id)
	return id, slug, err
}

// ListArticles: GET /api/articles — hanya PUBLISHED yang tampil publik.
func ListArticles(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		limit, offset, page, per := pageOf(r)
		var total int
		if err := db.QueryRow(r.Context(), `SELECT COUNT(*) FROM article WHERE status='PUBLISHED' AND ($1 = '' OR title ILIKE '%'||$1||'%')`, q).Scan(&total); err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		rows, err := db.Query(r.Context(),
			`SELECT a.id, a.title, a.slug, COALESCE(a.excerpt,''), COALESCE(a.image,''),
			        COALESCE(a.published_at, a.created_at), a.is_admin_post, COALESCE(u.name,'Redaksi')
			   FROM article a LEFT JOIN "user" u ON u.id = a.author_id
			  WHERE a.status = 'PUBLISHED' AND ($1 = '' OR a.title ILIKE '%'||$1||'%')
			  ORDER BY a.published_at DESC NULLS LAST LIMIT $2 OFFSET $3`, q, limit, offset)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type a struct {
			ID        string    `json:"id"`
			Title     string    `json:"title"`
			Slug      string    `json:"slug"`
			Excerpt   string    `json:"excerpt"`
			Image     string    `json:"image,omitempty"`
			Published time.Time `json:"published_at"`
			IsAdmin   bool      `json:"is_admin_post"`
			Author    string    `json:"author"`
		}
		out := []a{}
		for rows.Next() {
			var x a
			if err := rows.Scan(&x.ID, &x.Title, &x.Slug, &x.Excerpt, &x.Image, &x.Published, &x.IsAdmin, &x.Author); err != nil {
				continue
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in ListArticles: %v", err)
		}
		more := len(out) > per
		if more {
			out = out[:len(out)-1]
		}
		ok(w, map[string]any{"articles": out, "total": total, "page": page, "has_more": more})
	}
}

// GetArticle: GET /api/articles/{slug}. Artikel PENDING tidak bisa dibaca lewat
// URL langsung — penulis bisa membagikan drafnya sebelum lolos review.
func GetArticle(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var a struct {
			ID        string    `json:"id"`
			Title     string    `json:"title"`
			Slug      string    `json:"slug"`
			Content   string    `json:"content"`
			Excerpt   string    `json:"excerpt"`
			Image     string    `json:"image,omitempty"`
			Published time.Time `json:"published_at"`
			Author    string    `json:"author"`
		}
		err := db.QueryRow(r.Context(),
			`SELECT a.id, a.title, a.slug, a.content, COALESCE(a.excerpt,''), COALESCE(a.image,''),
			        COALESCE(a.published_at, a.created_at), COALESCE(u.name,'Redaksi')
			   FROM article a LEFT JOIN "user" u ON u.id = a.author_id
			  WHERE a.slug=$1 AND a.status='PUBLISHED'`, r.PathValue("slug")).
			Scan(&a.ID, &a.Title, &a.Slug, &a.Content, &a.Excerpt, &a.Image, &a.Published, &a.Author)
		if err != nil {
			fail(w, http.StatusNotFound, "artikel tidak ditemukan")
			return
		}
		a.Content = sanitizeArticleHTML(a.Content)
		ok(w, map[string]any{"article": a})
	}
}

// WriteArticle: POST /api/articles — kiriman user masuk PENDING (antre review).
func WriteArticle(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			Title   string `json:"title"`
			Content string `json:"content"`
			Excerpt string `json:"excerpt"`
			Image   string `json:"image"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		in.Content = sanitizeArticleHTML(in.Content)
		if len(strings.TrimSpace(in.Title)) < 3 {
			fail(w, http.StatusBadRequest, "judul minimal 3 karakter")
			return
		}
		if len(strings.TrimSpace(in.Content)) < 20 {
			fail(w, http.StatusBadRequest, "isi artikel minimal 20 karakter")
			return
		}
		// Link eksternal diperiksa jalur yang sama dengan produk (satu aturan).
		if v := detectLinks(in.Title + "\n" + in.Content + "\n" + in.Excerpt); len(v) > 0 {
			fail(w, http.StatusBadRequest, "link tidak diizinkan: "+strings.Join(v, " | "))
			return
		}
		role := dbRole(r.Context(), db, uid)
		id, slug, err := insertArticle(r.Context(), db, uid, role, false, strings.TrimSpace(in.Title), in.Content, in.Excerpt, in.Image, false)
		if err != nil {
			log.Printf("[ERROR] tulis artikel gagal: %v", err)
			fail(w, http.StatusInternalServerError, "gagal menyimpan artikel")
			return
		}
		ok(w, map[string]any{"ok": true, "id": id, "slug": slug, "status": "PENDING"})
	}
}

// ------------------------------------------------------------------ LINK GUARD

// Aturan link eksternal dipakai bersama produk dan artikel: satu daftar pola,
// jadi tidak mungkin ada satu jalur yang lolos karena lupa diperbarui.
//
// ponytail: regex, bukan parser URL penuh. Cukup untuk menandai konten
// mencurigakan; kalau nanti ada homoglyph/base64, eskalasi ke normalisasi.
var linkPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:https?|ftp)://\S+`),
	regexp.MustCompile(`(?i)\b(?:javascript|vbscript|data):\S+`),
	regexp.MustCompile(`(?i)\bwww\.\S+`),
	regexp.MustCompile(`(?i)\b[a-z0-9][a-z0-9-]*(?:\.[a-z0-9-]+)*\.(?:com|net|org|id|co|io|dev|app|me|xyz|info|biz|ru|cn|tk|ml|ga|cf|gq|link|site|online|store|shop|top|cc|tv|ly|be|to|ws|pw|su|gl|gd|win|bid|loan|click|download|stream|icu|cfd|sbs|monster|lol|rest)\b(?:/\S*)?`),
	regexp.MustCompile(`(?i)[a-z0-9]\s*(?:\[\s*\.\s*\]|\(\s*dot\s*\)|\{\s*\.\s*\})\s*[a-z]{2,}`),
}

func detectLinks(text string) []string {
	if text == "" {
		return nil
	}
	out := []string{}
	for _, re := range linkPatterns {
		for _, m := range re.FindAllString(text, 3) {
			// Host sendiri tidak dianggap pelanggaran.
			host := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(m, "www."), "http://"))
			if strings.HasPrefix(host, "aisell") || strings.HasPrefix(host, "localhost") {
				continue
			}
			out = append(out, m[:min(len(m), 60)])
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ------------------------------------------------------------------ util

// writeJSONBody dipakai handler admin yang meneruskan body JSON mentah.
func writeJSONBody(w http.ResponseWriter, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		fail(w, http.StatusInternalServerError, "encode error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.Copy(w, &buf)
}

var _ = fmt.Sprintf
