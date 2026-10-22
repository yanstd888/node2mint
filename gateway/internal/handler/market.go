package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Marketplace: katalog, keranjang, checkout escrow, pesanan.
//
// Aturan uangnya ada di SATU tempat (checkoutOrder + releaseEscrow +
// refundOrder) karena tiga jalur memakainya: beli langsung, checkout keranjang,
// dan tombol admin. Kalau dipisah, angka fee/payout bisa menyimpang antar jalur.

func ListProducts(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, page, per := pageOf(r)
		// Filter opsional: q (cari di judul+deskripsi) dan category (id ATAU slug).
		// Keduanya publik — katalog anonim harus bisa dicari/disaring.
		wheres := []string{"p.is_active = true", "COALESCE(s.is_active, true) = true", "p.deleted_at IS NULL"}
		args := []any{}
		if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
			args = append(args, "%"+q+"%")
			n := len(args)
			wheres = append(wheres, fmt.Sprintf("(p.title ILIKE $%d OR p.description ILIKE $%d)", n, n))
		}
		if cat := strings.TrimSpace(r.URL.Query().Get("category")); cat != "" {
			args = append(args, cat)
			n := len(args)
			wheres = append(wheres, fmt.Sprintf("(p.category_id::text = $%d OR p.category_id IN (SELECT id FROM market_category WHERE slug = $%d))", n, n))
		}
		if r.URL.Query().Get("verified") == "true" {
			wheres = append(wheres, "COALESCE((s.is_verified AND (s.verified_until IS NULL OR s.verified_until > now())), false)")
		}
		if r.URL.Query().Get("affiliate") == "true" {
			wheres = append(wheres, "p.allow_affiliate = true", "p.affiliate_percent > 0")
		}
		for _, filter := range []struct{ name, op string }{{"min_price", ">="}, {"max_price", "<="}} {
			raw := strings.TrimSpace(r.URL.Query().Get(filter.name))
			if raw == "" {
				continue
			}
			price, err := strconv.ParseFloat(raw, 64)
			if err != nil || math.IsNaN(price) || math.IsInf(price, 0) || price < 0 {
				fail(w, http.StatusBadRequest, filter.name+" tidak valid")
				return
			}
			args = append(args, price)
			wheres = append(wheres, fmt.Sprintf("p.price %s $%d", filter.op, len(args)))
		}
		if r.URL.Query().Get("instant") == "true" {
			wheres = append(wheres, "p.is_instant = true")
		}
		if r.URL.Query().Get("available") == "true" {
			wheres = append(wheres, "p.stock > 0")
		}
		if r.URL.Query().Get("sort") == "best_selling" {
			wheres = append(wheres, `(
				COALESCE(lower(su.role) IN ('admin','super_admin'), false)
				OR (p.is_boosted = true AND p.boosted_until > now())
			)`)
		}
		orderBy := "p.created_at DESC"
		switch r.URL.Query().Get("sort") {
		case "best_selling":
			orderBy = `p.sales_count DESC,
				CASE WHEN COALESCE(lower(su.role) IN ('admin','super_admin'), false) THEN 0 ELSE 1 END,
				CASE WHEN p.is_boosted = true AND p.boosted_until > now() THEN 0 ELSE 1 END,
				p.created_at DESC`
		case "price_low":
			orderBy = "p.price ASC, p.created_at DESC"
		case "price_high":
			orderBy = "p.price DESC, p.created_at DESC"
		}
		args = append(args, limit, offset)
		rows, err := db.Query(r.Context(),
			fmt.Sprintf(`SELECT p.id, p.seller_id, COALESCE(p.store_id, p.seller_id), p.title, COALESCE(p.description,''), p.price, p.stock, p.is_demo, p.is_instant, p.sales_count, COALESCE((SELECT COUNT(*) FROM market_review mr WHERE mr.product_id=p.id),0), COALESCE(p.image_url,''), COALESCE(s.name,'') AS store_name, COALESCE((s.is_verified AND (s.verified_until IS NULL OR s.verified_until > now())), false), COALESCE(lower(su.role) IN ('admin','super_admin'), false), p.allow_affiliate, p.affiliate_percent, p.created_at FROM market_product p LEFT JOIN market_store s ON s.id = p.store_id LEFT JOIN "user" su ON su.id = s.user_id WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`,
				strings.Join(wheres, " AND "), orderBy, len(args)-1, len(args)), args...)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type p struct {
			ID               string    `json:"id"`
			SellerID         string    `json:"seller_id"`
			StoreID          string    `json:"store_id"`
			Title            string    `json:"title"`
			Description      string    `json:"description"`
			Price            float64   `json:"price"`
			Stock            int       `json:"stock"`
			IsDemo           bool      `json:"is_demo"`
			IsInstant        bool      `json:"is_instant"`
			SalesCount       int       `json:"sales_count"`
			ReviewCount      int       `json:"review_count"`
			ImageURL         string    `json:"image_url,omitempty"`
			StoreName        string    `json:"store_name,omitempty"`
			StoreVerified    bool      `json:"store_verified"`
			StoreAdmin       bool      `json:"store_admin"`
			AllowAffiliate   bool      `json:"allow_affiliate"`
			AffiliatePercent float64   `json:"affiliate_percent"`
			CreatedAt        time.Time `json:"created_at"`
		}
		out := []p{}
		for rows.Next() {
			var x p
			if err := rows.Scan(&x.ID, &x.SellerID, &x.StoreID, &x.Title, &x.Description, &x.Price, &x.Stock, &x.IsDemo, &x.IsInstant, &x.SalesCount, &x.ReviewCount, &x.ImageURL, &x.StoreName, &x.StoreVerified, &x.StoreAdmin, &x.AllowAffiliate, &x.AffiliatePercent, &x.CreatedAt); err != nil {
				log.Printf("[WARN] scan error in ListProducts: %v", err)
				continue
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in ListProducts: %v", err)
		}
		more := len(out) > per
		if more {
			out = out[:len(out)-1]
		}
		ok(w, map[string]any{"products": out, "page": page, "has_more": more})
	}
}

// HomeFeatured: GET /api/market/featured — Marketplace pilihan berisi produk
// admin atau produk seller dengan boost yang masih aktif. Pin/sales/date hanya
// mengatur urutan setelah dua kelompok itu.
func HomeFeatured(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		limit := 12
		// Klien landing memakai per_page; limit didukung untuk pemakaian langsung.
		raw := r.URL.Query().Get("per_page")
		if raw == "" {
			raw = r.URL.Query().Get("limit")
		}
		if raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 36 {
				limit = n
			}
		}
		rows, err := db.Query(ctx,
			`SELECT p.id, p.seller_id, COALESCE(p.store_id, p.seller_id), p.title,
			       COALESCE(p.description,''), p.price, p.stock, p.is_demo, p.is_instant, p.sales_count,
			       COALESCE((SELECT COUNT(*) FROM market_review mr WHERE mr.product_id=p.id),0), COALESCE(p.image_url,''),
			       COALESCE(s.name,'') AS store_name,
			       COALESCE((s.is_verified AND (s.verified_until IS NULL OR s.verified_until > now())), false),
			       COALESCE(lower(su.role) IN ('admin','super_admin'), false), p.created_at
			  FROM market_product p JOIN market_store s ON s.id = p.store_id
			  LEFT JOIN "user" su ON su.id = s.user_id
			 WHERE p.is_active = true AND s.is_active = true AND p.deleted_at IS NULL
			   AND (
				 COALESCE(lower(su.role) IN ('admin','super_admin'), false)
				 OR (p.is_boosted = true AND p.boosted_until > now())
			   )
			 ORDER BY
			   CASE WHEN COALESCE(lower(su.role) IN ('admin','super_admin'), false) THEN 0 ELSE 1 END,
			   p.pin_order ASC NULLS LAST, p.sales_count DESC, p.created_at DESC
			 LIMIT $1`, limit)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type p struct {
			ID            string    `json:"id"`
			SellerID      string    `json:"seller_id"`
			StoreID       string    `json:"store_id"`
			Title         string    `json:"title"`
			Desc          string    `json:"description"`
			Price         float64   `json:"price"`
			Stock         int       `json:"stock"`
			IsDemo        bool      `json:"is_demo"`
			IsInstant     bool      `json:"is_instant"`
			SalesCount    int       `json:"sales_count"`
			ReviewCount   int       `json:"review_count"`
			ImageURL      string    `json:"image_url,omitempty"`
			StoreName     string    `json:"store_name,omitempty"`
			StoreVerified bool      `json:"store_verified"`
			StoreAdmin    bool      `json:"store_admin"`
			CreatedAt     time.Time `json:"created_at"`
		}
		out := []p{}
		for rows.Next() {
			var x p
			if err := rows.Scan(&x.ID, &x.SellerID, &x.StoreID, &x.Title, &x.Desc, &x.Price, &x.Stock, &x.IsDemo, &x.IsInstant, &x.SalesCount, &x.ReviewCount, &x.ImageURL, &x.StoreName, &x.StoreVerified, &x.StoreAdmin, &x.CreatedAt); err != nil {
				log.Printf("[WARN] scan error in HomeFeatured: %v", err)
				continue
			}
			out = append(out, x)
		}
		ok(w, map[string]any{"products": out})
	}
}

// GetProduct: GET /api/market/products/{id} — satu produk untuk halaman detail.
// Halaman /market yang memfilter dari daftar akan salah begitu katalog
// dipagination, jadi detail harus punya endpoint sendiri (parity web lama).
func GetProduct(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var x struct {
			ID          string    `json:"id"`
			SellerID    string    `json:"seller_id"`
			StoreID     string    `json:"store_id"`
			Title       string    `json:"title"`
			Description string    `json:"description"`
			Price       float64   `json:"price"`
			Stock       int       `json:"stock"`
			IsDemo      bool      `json:"is_demo"`
			IsInstant   bool      `json:"is_instant"`
			SalesCount  int       `json:"sales_count"`
			ReviewCount int       `json:"review_count"`
			ImageURL    string    `json:"image_url,omitempty"`
			CreatedAt   time.Time `json:"created_at"`
			Store       *struct {
				Name       string `json:"name"`
				IsVerified bool   `json:"is_verified"`
				IsAdmin    bool   `json:"is_admin"`
			} `json:"store,omitempty"`
		}
		var storeName *string
		var storeVerified, storeAdmin bool
		err := db.QueryRow(r.Context(),
			`SELECT p.id, p.seller_id, COALESCE(p.store_id, p.seller_id), p.title, COALESCE(p.description,''), p.price, p.stock, p.is_demo, p.is_instant, p.sales_count, COALESCE((SELECT COUNT(*) FROM market_review mr WHERE mr.product_id=p.id),0), COALESCE(p.image_url,''), p.created_at, s.name, COALESCE((s.is_verified AND (s.verified_until IS NULL OR s.verified_until > now())), false), COALESCE(lower(u.role) IN ('admin','super_admin'), false)
			   FROM market_product p LEFT JOIN market_store s ON s.id = p.store_id LEFT JOIN "user" u ON u.id=s.user_id
			  WHERE p.id=$1 AND p.is_active = true AND p.deleted_at IS NULL AND COALESCE(s.is_active, true) = true`, r.PathValue("id")).
			Scan(&x.ID, &x.SellerID, &x.StoreID, &x.Title, &x.Description, &x.Price, &x.Stock, &x.IsDemo, &x.IsInstant, &x.SalesCount, &x.ReviewCount, &x.ImageURL, &x.CreatedAt, &storeName, &storeVerified, &storeAdmin)
		if err != nil {
			fail(w, http.StatusNotFound, "produk tidak ditemukan")
			return
		}
		if storeName != nil {
			x.Store = &struct {
				Name       string `json:"name"`
				IsVerified bool   `json:"is_verified"`
				IsAdmin    bool   `json:"is_admin"`
			}{*storeName, storeVerified, storeAdmin}
		}
		ok(w, map[string]any{"product": x})
	}
}

// CreateOrder adalah bentuk lama (satu produk per order) yang masih dipakai
// halaman /market. Isinya hanya pembungkus checkoutOrder supaya tidak ada dua
// perhitungan uang.
func CreateOrder(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ProductID string `json:"product_id"`
			Qty       int    `json:"qty"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		requestKey, valid := idempotencyKey(w, r)
		if !valid {
			return
		}
		uid := uidFromCtx(r.Context())
		res := checkoutOrder(r, db, uid, false, []orderLine{{ProductID: in.ProductID, Qty: in.Qty}}, "", requestKey,
			func(x orderResult) any {
				return map[string]any{"order_id": x.Orders[0].ID, "status": "PAID", "total": x.Total}
			})
		if res.IdempotencyError != nil {
			failIdempotency(w, res.IdempotencyError)
			return
		}
		if res.CachedResponse != nil {
			replayIdempotency(w, res.CachedResponse)
			return
		}
		if !res.OK {
			fail(w, res.Code, res.Reason)
			return
		}
		ok(w, map[string]any{
			"order_id": res.Orders[0].ID, "status": "PAID", "total": res.Total,
		})
	}
}

type orderLine struct {
	CartID    string
	ProductID string
	Qty       int
}

type orderResult struct {
	OK               bool
	Code             int
	Reason           string
	Orders           []orderOut
	Total            float64
	Items            int
	CachedResponse   json.RawMessage
	IdempotencyError error
}

type orderOut struct {
	ID       string  `json:"id"`
	Total    float64 `json:"total"`
	SellerID string  `json:"-"`
	Title    string  `json:"-"`
}

// computeFee membagi uang: subtotal → fee platform (dari platform_setting) →
// seller_payout. Dibulatkan ke rupiah penuh: tidak ada pecahan sen di ledger.
func computeFee(ctx context.Context, db DB, subtotal float64) (fee, payout float64) {
	pct := settingFloat(ctx, db, "market_fee_percent", 10)
	fee = float64(int64(subtotal*pct/100 + 0.5))
	return fee, subtotal - fee
}

func affiliateCommission(net, percent float64) float64 {
	if net <= 0 || percent <= 0 || percent > 100 {
		return 0
	}
	return float64(int64(net*percent/100 + 0.5))
}

// sellerMayMarkDelivery: hanya order non-instan yang belum selesai, dan hanya
// sebelum batas kirim 24 jam lewat. Tanpa cek batas, seller bisa menandai
// "terkirim" SETELAH deadline: order keluar dari jangkauan auto-cancel
// (reconcile hanya menyasar PAID/PENDING), lalu escrow cair sendiri 2x24 jam
// kemudian — pembeli membayar barang yang tidak pernah dikirim.
func sellerMayMarkDelivery(status string, isInstant bool, deadline *time.Time, now time.Time) bool {
	if isInstant || (status != "PAID" && status != "PROCESSING") {
		return false
	}
	// Order PROCESSING sudah punya received_at dan tidak bisa lagi dibatalkan
	// reconcile, jadi batas kirim tidak berlaku untuk penandaan ulang.
	if status == "PROCESSING" {
		return true
	}
	return deadline == nil || deadline.After(now)
}

func autoReleaseEligible(status string, receivedAt *time.Time, hasOpenDispute bool, now time.Time) bool {
	return !hasOpenDispute && receivedAt != nil &&
		(status == "PAID" || status == "PROCESSING") &&
		!receivedAt.After(now.Add(-48*time.Hour))
}

// checkoutOrder: satu-satunya jalur yang memindahkan uang PEMBELI ke escrow.
//
// Order + potong saldo + potong stok berada dalam SATU transaksi, dan
// pemotongan saldo memakai syarat `>=` DI DALAM query — bukan cek-lalu-update —
// supaya dua checkout bersamaan tidak bisa sama-sama lolos.
func checkoutOrder(r *http.Request, db DB, uid string, cart bool, lines []orderLine, affiliateCode, requestKey string, response func(orderResult) any) orderResult {
	ctx := r.Context()
	if !cart && len(lines) == 0 {
		return orderResult{Code: http.StatusBadRequest, Reason: "tidak ada item untuk dibeli"}
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return orderResult{Code: http.StatusInternalServerError, Reason: "gagal memulai transaksi"}
	}
	defer tx.Rollback(ctx)

	intentBytes, err := json.Marshal(struct {
		Cart  bool        `json:"cart"`
		Lines []orderLine `json:"lines,omitempty"`
		Ref   string      `json:"ref,omitempty"`
	}{Cart: cart, Lines: lines, Ref: affiliateCode})
	if err != nil {
		return orderResult{Code: http.StatusInternalServerError, Reason: "gagal menyiapkan request"}
	}
	cached, err := claimIdempotency(ctx, tx, uid, "market_order", requestKey, string(intentBytes))
	if err != nil {
		return orderResult{IdempotencyError: err}
	}
	if cached != nil {
		return orderResult{CachedResponse: cached}
	}

	var clearCartIDs []string
	if cart {
		rows, err := tx.Query(ctx,
			`SELECT c.id, c.product_id, c.quantity FROM cart_item c WHERE c.user_id=$1 ORDER BY c.created_at`, uid)
		if err != nil {
			return orderResult{Code: http.StatusInternalServerError, Reason: "query keranjang gagal"}
		}
		for rows.Next() {
			var l orderLine
			if err := rows.Scan(&l.CartID, &l.ProductID, &l.Qty); err != nil {
				rows.Close()
				return orderResult{Code: http.StatusInternalServerError, Reason: "gagal membaca keranjang"}
			}
			lines = append(lines, l)
			clearCartIDs = append(clearCartIDs, l.CartID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return orderResult{Code: http.StatusInternalServerError, Reason: "gagal membaca keranjang"}
		}
		rows.Close()
		if len(lines) == 0 {
			return orderResult{Code: http.StatusBadRequest, Reason: "keranjang kosong"}
		}
	}

	var res orderResult
	res.OK = true
	for _, l := range lines {
		qty := l.Qty
		if qty < 1 {
			// Field qty tidak dikenal/0/negatif tidak boleh diam-diam jadi 1 —
			// client harus mengirim jumlah yang jelas.
			return orderResult{Code: http.StatusBadRequest, Reason: "jumlah (qty) minimal 1"}
		}

		// Kunci baris produk: cek stok dan potong stok tidak boleh terpisah.
		// owner = pemilik toko (atau seller tunggal) — untuk blokir beli produk
		// sendiri; store_active menutup toko yang belum disetujui admin.
		var title, storeID, ownerID, storeName string
		var price float64
		var stock, lowStockThreshold int
		var lowStockNotified bool
		var isActive, isDemo bool
		var allowAff, storeActive, isInstant bool
		var affiliatePercent float64
		err := tx.QueryRow(ctx,
			`SELECT p.title, COALESCE(p.store_id::text,''), p.price, p.stock, p.is_active, p.is_demo, p.allow_affiliate, p.affiliate_percent,
			        COALESCE(s.user_id::text, p.seller_id::text), COALESCE(s.is_active, true), p.is_instant,
			        COALESCE(s.name,''), COALESCE(p.low_stock_threshold,3), COALESCE(p.low_stock_notified,false)
			   FROM market_product p LEFT JOIN market_store s ON s.id = p.store_id
			  WHERE p.id=$1 AND p.deleted_at IS NULL FOR UPDATE OF p`, l.ProductID).
			Scan(&title, &storeID, &price, &stock, &isActive, &isDemo, &allowAff, &affiliatePercent, &ownerID, &storeActive, &isInstant, &storeName, &lowStockThreshold, &lowStockNotified)
		if err != nil {
			return orderResult{Code: http.StatusNotFound, Reason: "produk tidak ditemukan"}
		}
		if !isActive {
			return orderResult{Code: http.StatusBadRequest, Reason: fmt.Sprintf("Produk %q sedang tidak ditayangkan", title)}
		}
		if isDemo {
			return orderResult{Code: http.StatusBadRequest, Reason: fmt.Sprintf("Produk demo %q tidak dapat dibeli", title)}
		}
		if !storeActive {
			return orderResult{Code: http.StatusBadRequest, Reason: fmt.Sprintf("Toko produk %q sedang tidak aktif", title)}
		}
		if ownerID == uid {
			return orderResult{Code: http.StatusBadRequest, Reason: "tidak bisa membeli produk sendiri"}
		}
		if stock < qty {
			return orderResult{Code: http.StatusBadRequest, Reason: fmt.Sprintf("Stok %q hanya %d", title, stock)}
		}
		// Produk instan: yang dikirim adalah baris market_product_item, bukan
		// angka stok. Baris yang diserahkan harus PERSIS sebanyak qty; jumlah
		// stok yang tidak cocok dengan barang nyata lebih baik menolak order
		// daripada menagih pembeli untuk barang yang tidak ada.
		if isInstant {
			var siap int
			if err := tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM market_product_item WHERE product_id=$1 AND order_id IS NULL`, l.ProductID).Scan(&siap); err != nil {
				return orderResult{Code: http.StatusInternalServerError, Reason: "gagal memeriksa isian instan"}
			}
			if siap != stock || siap < qty {
				return orderResult{Code: http.StatusBadRequest, Reason: fmt.Sprintf("Stok instan %q tidak konsisten (%d barang siap kirim)", title, siap)}
			}
		}

		subtotal := price * float64(qty)
		fee, payout := computeFee(ctx, db, subtotal)

		// Kode afiliasi hanya diterima kalau link aktif, produknya memang
		// mengaktifkan afiliasi, dan pembeli bukan pemilik link sendiri.
		code := ""
		commission := float64(0)
		if affiliateCode != "" && allowAff && affiliatePercent > 0 {
			var linkProduct, affiliator string
			if err := tx.QueryRow(ctx,
				`SELECT product_id::text, user_id::text FROM affiliate_link WHERE code=$1 AND is_active`, affiliateCode).
				Scan(&linkProduct, &affiliator); err == nil && linkProduct == l.ProductID && affiliator != uid {
				code = affiliateCode
				commission = affiliateCommission(payout, affiliatePercent)
				payout -= commission
			}
		}

		var orderID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO market_order (buyer_id, product_id, store_id, qty, total_price, price_at_time, subtotal,
			                           fee_amount, seller_payout, affiliate_code, affiliate_commission_amount, status, process_deadline, received_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'PAID', now() + interval '24 hours', CASE WHEN $12 THEN now() ELSE NULL END) RETURNING id`,
			uid, l.ProductID, nullStr(storeID), qty, subtotal, price, subtotal,
			fee, payout, nullStr(code), commission, isInstant).Scan(&orderID); err != nil {
			log.Printf("[ERROR] insert order gagal: user=%s err=%v", uid, err)
			return orderResult{Code: http.StatusInternalServerError, Reason: "gagal membuat order"}
		}

		// Saldo dipotong dengan syarat di dalam query. 0 baris = tidak cukup.
		cut, err := tx.Exec(ctx,
			`UPDATE "user" SET wallet_balance = wallet_balance - $1 WHERE id=$2 AND wallet_balance >= $1`,
			subtotal, uid)
		if err != nil {
			log.Printf("[ERROR] potong saldo gagal: user=%s err=%v", uid, err)
			return orderResult{Code: http.StatusInternalServerError, Reason: "gagal memproses pembayaran"}
		}
		if cut.RowsAffected() == 0 {
			return orderResult{Code: http.StatusPaymentRequired, Reason: "saldo tidak cukup"}
		}
		// Buku besar: satu baris per pemotongan, supaya mutasi di /wallet cocok
		// dengan saldo, bukan hanya berubah diam-diam.
		if _, err := tx.Exec(ctx,
			`INSERT INTO credit_transaction (user_id, amount, type, status, description, invoice_id)
			 VALUES ($1,$2,'USAGE_DEDUCT','COMPLETED',$3,$4)`,
			uid, -subtotal, "Pembelian "+title, orderID); err != nil {
			return orderResult{Code: http.StatusInternalServerError, Reason: "gagal mencatat mutasi"}
		}

		if isInstant {
			// Tepat qty baris dari yang belum terjual — bukan "semua yang
			// belum terjual". SKIP LOCKED wajib: tanpa itu, dua checkout
			// bersamaan sama-sama membaca baris yang sama, yang kedua menimpa
			// milik yang pertama, dan pembeli pertama sudah terlanjur dibayar.
			// Dengan SKIP LOCKED, checkout kedua melewatkan baris yang sedang
			// dikunci lalu kehabisan baris → ditolak (409), bukan ditagih.
			tag, err := tx.Exec(ctx,
				`UPDATE market_product_item SET order_id=$1 WHERE id IN (
				   SELECT id FROM market_product_item
				    WHERE product_id=$2 AND order_id IS NULL
				    ORDER BY created_at, id LIMIT $3 FOR UPDATE SKIP LOCKED)`,
				orderID, l.ProductID, qty)
			if err != nil {
				return orderResult{Code: http.StatusInternalServerError, Reason: "gagal menyiapkan barang instan"}
			}
			if tag.RowsAffected() != int64(qty) {
				return orderResult{Code: http.StatusConflict, Reason: fmt.Sprintf("Barang instan %q tidak lengkap (%d dari %d)", title, tag.RowsAffected(), qty)}
			}
		}

		newStock := stock - qty
		newLowStockNotified := lowStockNotified
		lowStockType := ""
		if newStock > lowStockThreshold {
			newLowStockNotified = false
		} else if !lowStockNotified {
			newLowStockNotified = true
			if newStock <= 0 {
				lowStockType = "OUT_OF_STOCK"
			} else {
				lowStockType = "LOW_STOCK"
			}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE market_product SET stock = stock - $1, low_stock_notified=$2 WHERE id=$3 AND stock >= $1`, qty, newLowStockNotified, l.ProductID); err != nil {
			return orderResult{Code: http.StatusInternalServerError, Reason: "gagal memperbarui stok"}
		}

		orderPayload := sellerTelegramPayload{Store: storeName, Title: title, Quantity: qty, Amount: subtotal, OrderID: orderID, ProductID: l.ProductID}
		if err := enqueueSellerTelegram(ctx, tx, "order:new:"+orderID, "NEW_ORDER", ownerID, orderID, l.ProductID, orderPayload); err != nil {
			log.Printf("[WARN] enqueue seller order notification gagal: %v", err)
		}
		if lowStockType != "" {
			stockPayload := orderPayload
			stockPayload.Stock = newStock
			stockPayload.Threshold = lowStockThreshold
			if err := enqueueSellerTelegram(ctx, tx, "stock:"+lowStockType+":"+l.ProductID+":"+orderID, lowStockType, ownerID, orderID, l.ProductID, stockPayload); err != nil {
				log.Printf("[WARN] enqueue seller stock notification gagal: %v", err)
			}
		}
		if err := enqueueEmailOutbox(ctx, tx, "market:"+orderID, emailEventMarket, uid, orderID); err != nil {
			log.Printf("[WARN] enqueue marketplace payment email gagal: %v", err)
		}

		res.Orders = append(res.Orders, orderOut{ID: orderID, Total: subtotal, SellerID: ownerID, Title: title})
		res.Total += subtotal
		res.Items += qty
	}

	for _, id := range clearCartIDs {
		if _, err := tx.Exec(ctx, `DELETE FROM cart_item WHERE id=$1 AND user_id=$2`, id, uid); err != nil {
			return orderResult{Code: http.StatusInternalServerError, Reason: "gagal memperbarui keranjang"}
		}
	}

	if err := completeIdempotency(ctx, tx, uid, "market_order", requestKey, response(res)); err != nil {
		return orderResult{IdempotencyError: err}
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("[ERROR] commit checkout gagal: %v", err)
		return orderResult{Code: http.StatusInternalServerError, Reason: "gagal memproses order"}
	}
	// Generate notifikasi per order (async, best-effort: gagal tidak batalkan order).
	for _, o := range res.Orders {
		go genPurchaseNotification(context.Background(), db, uid, o.ID, o.Title)
		go genSaleNotification(context.Background(), db, o.ID, o.SellerID, o.Title)
	}
	// CheckOrderVelocity: async, best-effort — fraud flagging must never block
	// or roll back a successful checkout.
	go CheckOrderVelocity(context.Background(), db, uid, ipOf(r))
	return res
}

// Checkout: POST /api/market/checkout {items[] | cart:true}.
func Checkout(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			Items []struct {
				ProductID string `json:"product_id"`
				Qty       int    `json:"qty"`
			} `json:"items"`
			Cart bool   `json:"cart"`
			Ref  string `json:"ref,omitempty"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}

		requestKey, valid := idempotencyKey(w, r)
		if !valid {
			return
		}

		// Body kosong ({}) = checkout keranjang, itu yang dikirim halaman
		// /keranjang. Isi keranjang dibaca di dalam transaksi checkout supaya
		// retry dengan key yang sama tetap dapat me-replay setelah cart dikosongkan.
		cart := in.Cart || len(in.Items) == 0
		var lines []orderLine
		if !cart {
			for _, it := range in.Items {
				if it.ProductID != "" {
					lines = append(lines, orderLine{ProductID: it.ProductID, Qty: it.Qty})
				}
			}
		}

		res := checkoutOrder(r, db, uid, cart, lines, in.Ref, requestKey,
			func(x orderResult) any {
				return map[string]any{
					"ok": true, "orders": x.Orders, "total": x.Total,
					"items": x.Items, "escrow": true,
				}
			})
		if res.IdempotencyError != nil {
			failIdempotency(w, res.IdempotencyError)
			return
		}
		if res.CachedResponse != nil {
			replayIdempotency(w, res.CachedResponse)
			return
		}
		if !res.OK {
			fail(w, res.Code, res.Reason)
			return
		}
		ok(w, map[string]any{
			"ok": true, "orders": res.Orders, "total": res.Total,
			"items": res.Items, "escrow": true,
		})
	}
}

// released = order + escrow + data yang dibutuhkan pencatat komisi.
type released struct {
	OrderID             string
	StoreID             string
	SellerPayout        float64
	FeeAmount           float64
	Qty                 int
	ProductID           string
	AffiliateCode       string
	AffiliateCommission float64
}

// scanIDs membaca satu kolom id sampai habis ke slice. Dipakai semua pemanggil
// yang setelahnya membuka transaksi per-id: cursor yang masih terbuka menahan
// satu koneksi pool, sementara tiap transaksi butuh koneksi kedua — cukup
// sedikit baris untuk membuat keduanya saling menunggu sampai pool habis.
func scanIDs(ctx context.Context, db DB, sql string, args ...any) ([]string, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// restockOrder mengembalikan stok produk + barang instan milik satu order
// (order_id dilepas supaya bisa dibeli lagi). Dipakai cancelOrder DAN refund
// sengketa buyer_wins: dua jalur yang sama-sama "batalkan order", jadi
// pengembalian barang harus satu implementasi — kalau dipisah, salah satu
// jalur cepat atau lambat lupa melepas barang instan, dan baris instan yang
// masih terikat membuat `siap != stock` sehingga produknya tidak bisa dibeli
// siapa pun lagi.
// Jumlah & tipe produk dibaca dari baris order sendiri, jadi pemanggil tidak
// bisa salah menyerahkan angka. Tepat sekali dipanggil per order: kedua
// pemanggil sudah memegang kunci baris order + guard status.
func restockOrder(ctx context.Context, tx pgx.Tx, orderID string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE market_product p SET stock = p.stock + o.qty,
			low_stock_notified = CASE WHEN p.stock + o.qty > COALESCE(p.low_stock_threshold,3) THEN false ELSE p.low_stock_notified END,
			updated_at=now()
		   FROM market_order o WHERE o.id=$1 AND p.id = o.product_id`, orderID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE market_product_item i SET order_id=NULL
		   FROM market_order o JOIN market_product p ON p.id = o.product_id
		  WHERE o.id=$1 AND p.is_instant AND i.order_id=$1 AND i.product_id=o.product_id`, orderID); err != nil {
		return err
	}
	return nil
}

// releaseEscrowAuto: pelepasan escrow otomatis (reconciler, tanpa aktor).
// Audit ditulis di dalam transaksi yang sama supaya pelepasan dana otomatis
// tetap meninggalkan jejak — kalau audit gagal, dana tidak berpindah.
func releaseEscrowAuto(ctx context.Context, db DB, orderID string) (released, string, bool) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return released{}, "gagal memulai transaksi", false
	}
	defer tx.Rollback(ctx)
	r, reason, fine := releaseEscrowTx(ctx, tx, orderID)
	if !fine {
		return released{}, reason, false
	}
	if err := auditTx(ctx, tx, "", "ORDER_AUTO_COMPLETE", orderID, "PAID", "COMPLETED", ""); err != nil {
		return released{}, "gagal mencatat audit", false
	}
	if err := tx.Commit(ctx); err != nil {
		return released{}, "gagal menyimpan penyelesaian order", false
	}
	return r, "", true
}

func releaseEscrowAudited(ctx context.Context, db DB, orderID, actorID, action, ip string) (released, string, bool) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return released{}, "gagal memulai transaksi", false
	}
	defer tx.Rollback(ctx)
	r, reason, fine := releaseEscrowTx(ctx, tx, orderID)
	if !fine {
		return released{}, reason, false
	}
	if err := auditTx(ctx, tx, actorID, action, orderID, "PAID", "COMPLETED", ip); err != nil {
		return released{}, "gagal mencatat audit", false
	}
	if err := tx.Commit(ctx); err != nil {
		return released{}, "gagal menyimpan penyelesaian order", false
	}
	return r, "", true
}

// releaseEscrowTx: inti pelepasan escrow pada transaksi milik pemanggil. Dipakai
// ResolveDispute untuk melanjutkan order seller-wins ATOMIK dengan penutupan
// sengketa — memanggil releaseEscrow (yang membuka transaksi sendiri) setelah
// commit akan meninggalkan celah: order sudah PAID tapi dana belum pindah, dan
// reconciler bisa membatalkannya lebih dulu.
func releaseEscrowTx(ctx context.Context, tx pgx.Tx, orderID string) (released, string, bool) {
	var r released
	var storeID, aff *string
	var status string
	var hold string
	err := tx.QueryRow(ctx,
		`SELECT id::text, store_id::text, seller_payout, fee_amount, qty, product_id::text, affiliate_code, affiliate_commission_amount, status,
		        COALESCE((SELECT status FROM market_dispute WHERE order_id = market_order.id AND status IN ('OPEN','MANUAL_REVIEW') LIMIT 1), '')
		   FROM market_order WHERE id=$1 FOR UPDATE`, orderID).
		Scan(&r.OrderID, &storeID, &r.SellerPayout, &r.FeeAmount, &r.Qty, &r.ProductID, &aff, &r.AffiliateCommission, &status, &hold)
	if err != nil {
		return released{}, "order tidak ditemukan", false
	}
	if status != "PAID" && status != "PROCESSING" {
		return released{}, "Order sudah selesai atau dibatalkan", false
	}
	if hold != "" {
		return released{}, "Order sedang disengketakan — dana ditahan sampai diputuskan", false
	}
	if storeID != nil {
		r.StoreID = *storeID
	}
	if aff != nil {
		r.AffiliateCode = *aff
	}

	// Tanpa toko tidak ada yang bisa dibayar. Order TIDAK boleh diam-diam
	// ditutup COMPLETED: pembeli sudah dibayar, dan menutupnya menghapus satu-
	// satunya jejak bahwa uang itu masih menggantung. Biarkan status apa adanya
	// supaya muncul di daftar order yang butuh tindakan admin.
	if r.StoreID == "" {
		return released{}, "Order tanpa toko — tidak ada penerima payout, perbaiki store_id order ini dulu", false
	}
	if _, err := tx.Exec(ctx,
		`UPDATE market_store SET wallet_balance = wallet_balance + $1, updated_at=now() WHERE id=$2`,
		r.SellerPayout, r.StoreID); err != nil {
		return released{}, "gagal membayar seller", false
	}
	if _, err := tx.Exec(ctx,
		`UPDATE market_product SET sales_count = sales_count + $1, updated_at=now() WHERE id=$2`,
		r.Qty, r.ProductID); err != nil {
		return released{}, "gagal memperbarui penjualan", false
	}
	if _, err := tx.Exec(ctx,
		`UPDATE market_order SET status='COMPLETED', completed_at=now(), updated_at=now() WHERE id=$1`, orderID); err != nil {
		return released{}, "gagal menutup order", false
	}

	// Komisi afiliasi sudah dihitung dan dipotong dari bagian seller saat
	// checkout, setelah fee platform. Nilai snapshot ini tidak berubah walau
	// seller mengubah persentase sebelum order diselesaikan.
	if r.AffiliateCode != "" && r.AffiliateCommission > 0 {
		var linkID, affiliator string
		if err := tx.QueryRow(ctx,
			`SELECT id::text, user_id::text FROM affiliate_link WHERE code=$1`, r.AffiliateCode).
			Scan(&linkID, &affiliator); err == nil {
			commission := r.AffiliateCommission
			if tag, err := tx.Exec(ctx,
				`INSERT INTO affiliate_conversion (link_id, order_id, commission_amount) VALUES ($1,$2,$3)
				 ON CONFLICT (order_id) DO NOTHING`, linkID, orderID, commission); err != nil {
				log.Printf("[WARN] catat konversi afiliasi gagal: %v", err)
			} else if tag.RowsAffected() == 1 {
				if _, err := tx.Exec(ctx,
					`UPDATE "user" SET wallet_balance = wallet_balance + $1 WHERE id=$2`, commission, affiliator); err != nil {
					return released{}, "gagal membayar komisi afiliasi", false
				}
				if _, err := tx.Exec(ctx,
					`INSERT INTO credit_transaction (user_id, amount, type, status, description)
					 VALUES ($1,$2,'REFERRAL_COMMISSION','COMPLETED',$3)`,
					affiliator, commission, "Komisi afiliasi order "+orderID); err != nil {
					return released{}, "gagal mencatat komisi afiliasi", false
				}
				// Notif dompet ditulis dalam transaksi yang sama — tepat di titik
				// kredit berhasil, jadi tak ada notif tanpa saldo (atau sebaliknya).
				// RowsAffected()==1 di atas = sudah idempoten per order.
				genAffiliateCommissionNotification(ctx, tx, affiliator, orderID, commission)
			}
		}
	}

	return r, "", true
}

// ReleaseDueEscrows releases only deliveries that have stayed undisputed for
// 2x24 hours. The order lock and releaseEscrow status guard make the scheduler
// safe to run every minute without paying a seller twice.
//
// Daftar id dibaca HABIS dulu ke slice, baru diproses: kalau loop berjalan
// sambil cursor terbuka, tiap releaseEscrow butuh koneksi kedua dari pool yang
// sama — cukup beberapa order bersamaan untuk saling menunggu koneksi.
func ReleaseDueEscrows(ctx context.Context, db DB) int {
	ids, err := scanIDs(ctx, db,
		`SELECT o.id
		   FROM market_order o
		  WHERE o.status IN ('PAID','PROCESSING')
		    AND o.store_id IS NOT NULL
		    AND o.received_at IS NOT NULL
		    AND o.received_at <= now() - interval '48 hours'
		    AND NOT EXISTS (
		      SELECT 1 FROM market_dispute d WHERE d.order_id=o.id AND d.status IN ('OPEN','MANUAL_REVIEW')
		    )
		  ORDER BY o.received_at ASC LIMIT 50`)
	if err != nil {
		log.Printf("[WARN] due escrow query failed: %v", err)
		return 0
	}

	released := 0
	for _, id := range ids {
		if _, _, ok := releaseEscrowAuto(ctx, db, id); ok {
			released++
		}
	}
	if released > 0 {
		log.Printf("[INFO] ReleaseDueEscrows: released %d orders", released)
	}
	return released
}

// MarkOrderDelivered starts the 2x24h escrow hold for a non-instant order.
// Only its store owner can declare delivery; a buyer confirmation can still
// release it immediately through ConfirmOrder.
func MarkOrderDelivered(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id, valid := pathUUID(w, r, "id")
		if !valid {
			return
		}

		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		defer tx.Rollback(r.Context())

		var status string
		var isInstant bool
		var deadline *time.Time
		err = tx.QueryRow(r.Context(),
			`SELECT o.status, p.is_instant, o.process_deadline
			   FROM market_order o
			   JOIN market_product p ON p.id=o.product_id
			   JOIN market_store s ON s.id=o.store_id
			  WHERE o.id=$1 AND s.user_id=$2 FOR UPDATE OF o`, id, uid).
			Scan(&status, &isInstant, &deadline)
		if err != nil {
			fail(w, http.StatusNotFound, "order tidak ditemukan atau bukan milik toko Anda")
			return
		}
		if !sellerMayMarkDelivery(status, isInstant, deadline, time.Now()) {
			fail(w, http.StatusConflict, "order tidak dapat ditandai terkirim — status tidak sesuai atau batas kirim 24 jam sudah lewat")
			return
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE market_order
			    SET status='PROCESSING', processed_at=COALESCE(processed_at, now()),
			        received_at=COALESCE(received_at, now()), updated_at=now()
			  WHERE id=$1`, id); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menandai pengiriman")
			return
		}
		if err := auditTx(r.Context(), tx, uid, "ORDER_DELIVERED", id, status, "PROCESSING", ipOf(r)); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mencatat audit")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan pengiriman")
			return
		}
		ok(w, map[string]any{"ok": true, "status": "PROCESSING"})
	}
}

// StoreOrders: GET /api/store/orders — pesanan yang MASUK ke toko sendiri
// (parity web lama /api/seller/orders). Termasuk seller_payout supaya penjual
// bisa melihat bagiannya; hanya pemilik toko yang bisa membaca.
func StoreOrders(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		s, found := loadStore(r.Context(), db, uid)
		if !found {
			fail(w, http.StatusNotFound, "toko belum terdaftar")
			return
		}
		limit, offset, page, per := pageOf(r)
		rows, err := db.Query(r.Context(),
			`SELECT o.id, o.qty, o.subtotal, o.seller_payout, o.status, o.created_at, o.process_deadline, o.completed_at, p.is_instant, p.title
			   FROM market_order o JOIN market_product p ON p.id = o.product_id
			  WHERE o.store_id = $1 ORDER BY o.created_at DESC LIMIT $2 OFFSET $3`, s.ID, limit, offset)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type so struct {
			ID              string     `json:"id"`
			Quantity        int        `json:"quantity"`
			Subtotal        float64    `json:"subtotal"`
			SellerPayout    float64    `json:"seller_payout"`
			Status          string     `json:"status"`
			CreatedAt       time.Time  `json:"created_at"`
			ProcessDeadline *time.Time `json:"process_deadline,omitempty"`
			CompletedAt     *time.Time `json:"completed_at,omitempty"`
			IsInstant       bool       `json:"is_instant"`
			ProductTitle    string     `json:"product_title"`
		}
		out := []so{}
		for rows.Next() {
			var x so
			var qty int
			if err := rows.Scan(&x.ID, &qty, &x.Subtotal, &x.SellerPayout, &x.Status, &x.CreatedAt, &x.ProcessDeadline, &x.CompletedAt, &x.IsInstant, &x.ProductTitle); err != nil {
				continue
			}
			x.Quantity = qty
			out = append(out, x)
		}
		more := len(out) > per
		if more {
			out = out[:len(out)-1]
		}
		ok(w, map[string]any{"orders": out, "page": page, "has_more": more})
	}
}

// BuyerOrders: GET /api/market/orders — pesanan milik pembeli yang login.
func BuyerOrders(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		limit, offset, page, per := pageOf(r)
		// Lazy auto-cancel: selama pembeli membuka halaman pesanan, selesaikan dulu
		// order yang melewati deadline 24h (refund + stok dikembalikan).
		ReconcileOverdueOrders(r.Context(), db)
		rows, err := db.Query(r.Context(),
			`SELECT o.id, o.qty, o.subtotal, o.status, o.created_at, o.process_deadline, o.completed_at, p.is_instant,
			        p.title, COALESCE(s.name,'')
			   FROM market_order o
			   JOIN market_product p ON p.id = o.product_id
			   LEFT JOIN market_store s ON s.id = o.store_id
			  WHERE o.buyer_id = $1 ORDER BY o.created_at DESC LIMIT $2 OFFSET $3`, uid, limit, offset)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type o struct {
			ID  string `json:"id"`
			Qty int    `json:"qty"`
			// Halaman /pesanan membaca quantity, product.title, store.name.
			Quantity        int        `json:"quantity"`
			Subtotal        float64    `json:"subtotal"`
			Status          string     `json:"status"`
			CreatedAt       time.Time  `json:"created_at"`
			ProcessDeadline *time.Time `json:"process_deadline,omitempty"`
			CompletedAt     *time.Time `json:"completed_at,omitempty"`
			IsInstant       bool       `json:"is_instant"`
			Title           string     `json:"product_title"`
			Product         struct {
				Title string `json:"title"`
			} `json:"product"`
			StoreName string `json:"store_name"`
			Store     struct {
				Name string `json:"name"`
			} `json:"store"`
		}
		out := []o{}
		for rows.Next() {
			var x o
			if err := rows.Scan(&x.ID, &x.Qty, &x.Subtotal, &x.Status, &x.CreatedAt, &x.ProcessDeadline, &x.CompletedAt, &x.IsInstant, &x.Title, &x.StoreName); err != nil {
				continue
			}
			x.Quantity = x.Qty
			x.Product.Title = x.Title
			x.Store.Name = x.StoreName
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in BuyerOrders: %v", err)
		}
		more := len(out) > per
		if more {
			out = out[:len(out)-1]
		}
		ok(w, map[string]any{"orders": out, "page": page, "has_more": more})
	}
}

// BuyerOrderItems: GET /api/market/orders/{id}/items — isi barang instan untuk
// SATU pesanan milik pembeli yang meminta. Barang yang sudah dibeli hanya boleh
// dibaca pembelinya (dan admin); tanpa syarat ini satu id order membocorkan
// isi pembelian orang lain.
func BuyerOrderItems(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id, valid := pathUUID(w, r, "id")
		if !valid {
			return
		}
		var owner string
		var status string
		if err := db.QueryRow(r.Context(),
			`SELECT COALESCE(buyer_id::text,''), status FROM market_order WHERE id=$1`, id).Scan(&owner, &status); err != nil {
			fail(w, http.StatusNotFound, "order tidak ditemukan")
			return
		}
		if owner != uid && !isAdminRole(dbRole(r.Context(), db, uid)) {
			fail(w, http.StatusForbidden, "bukan pesanan Anda")
			return
		}
		rows, err := db.Query(r.Context(),
			`SELECT content FROM market_product_item WHERE order_id=$1 ORDER BY created_at, id`, id)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		items := []string{}
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				continue
			}
			// Isian tersimpan terenkripsi (shop.go: encryptInstantItems);
			// hanya pembeli/admin yang sampai ke sini, jadi plaintext aman keluar.
			items = append(items, decryptInstantItem(c))
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in BuyerOrderItems: %v", err)
		}
		ok(w, map[string]any{"order_id": id, "items": items, "count": len(items)})
	}
}

// ConfirmOrder: pembeli menutup pesanan dan melepas escrow ke seller.
func ConfirmOrder(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")

		var owner string
		var status string
		if err := db.QueryRow(r.Context(),
			`SELECT COALESCE(buyer_id::text,''), status FROM market_order WHERE id=$1`, id).Scan(&owner, &status); err != nil {
			fail(w, http.StatusNotFound, "order tidak ditemukan")
			return
		}
		if owner != uid {
			fail(w, http.StatusForbidden, "bukan pesanan Anda")
			return
		}
		res, reason, fine := releaseEscrowAudited(r.Context(), db, id, uid, "ORDER_CONFIRM", ipOf(r))
		if !fine {
			fail(w, http.StatusConflict, reason)
			return
		}
		ok(w, map[string]any{"ok": true, "status": "COMPLETED", "seller_payout": res.SellerPayout})
	}
}

// CancelOrder: pembatalan (pembeli atau admin) → saldo pembeli kembali, stok
// dikembalikan, dan ledger mencatatnya.
func CancelOrder(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		admin := isAdminRole(dbRole(r.Context(), db, uid))
		id := r.PathValue("id")

		code, reason := cancelOrder(r.Context(), db, id, uid, admin, "", "ORDER_CANCEL", ipOf(r))
		if code != 0 {
			fail(w, code, reason)
			return
		}
		ok(w, map[string]any{"ok": true, "status": "CANCELLED"})
	}
}

// cancelOrder: inti pembatalan. admin = boleh membatalkan order siapa pun.
//
// Sengketa adalah keputusan admin, bukan keputusan salah satu pihak: order yang
// sedang disengketakan tidak bisa dibatalkan lewat jalur biasa (pembeli/admin
// tombol refund) — kalau lolos, refund pembatalan + putusan sengketa bisa
// membayar dua kali dari sumber berbeda. Jalur yang benar: putuskan sengketanya.
func cancelOrder(ctx context.Context, db DB, orderID, actorID string, admin bool, note, auditAction, auditIP string) (int, string) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return http.StatusInternalServerError, "gagal memulai transaksi"
	}
	defer tx.Rollback(ctx)

	var buyer, status string
	var subtotal float64
	var hold string
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(o.buyer_id::text,''), o.status, o.subtotal,
		        COALESCE((SELECT d.status FROM market_dispute d WHERE d.order_id=o.id AND d.status IN ('OPEN','MANUAL_REVIEW') LIMIT 1),'')
		   FROM market_order o JOIN market_product p ON p.id = o.product_id
		  WHERE o.id=$1 FOR UPDATE OF o`, orderID).
		Scan(&buyer, &status, &subtotal, &hold)
	if err != nil {
		return http.StatusNotFound, "order tidak ditemukan"
	}
	if !admin && buyer != actorID {
		return http.StatusForbidden, "bukan pesanan Anda"
	}
	// Dana yang sudah dilepas ke seller tidak bisa ditarik dari sini — escrow
	// yang sudah cair harus diselesaikan lewat refund admin.
	if status != "PAID" && status != "PENDING" && status != "PROCESSING" {
		return http.StatusConflict, "Order sudah selesai atau dibatalkan"
	}
	if hold != "" {
		return http.StatusConflict, "Order sedang disengketakan — putuskan sengketanya dulu"
	}

	if _, err := tx.Exec(ctx,
		`UPDATE market_order SET status='CANCELLED', refunded_at=now(), updated_at=now() WHERE id=$1`, orderID); err != nil {
		return http.StatusInternalServerError, "gagal membatalkan order"
	}
	// Stok + barang instan kembali dalam transaksi yang sama dengan refund.
	if err := restockOrder(ctx, tx, orderID); err != nil {
		return http.StatusInternalServerError, "gagal mengembalikan stok"
	}
	if buyer != "" && subtotal > 0 {
		desc := "Refund pembatalan pesanan"
		if note != "" {
			desc = note
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO credit_transaction (user_id, amount, type, status, description, invoice_id)
			 VALUES ($1,$2,'REFUND','COMPLETED',$3,$4)`, buyer, subtotal, desc, orderID); err != nil {
			return http.StatusInternalServerError, "gagal mencatat refund"
		}
		if _, err := tx.Exec(ctx,
			`UPDATE "user" SET wallet_balance = wallet_balance + $1 WHERE id=$2`, subtotal, buyer); err != nil {
			return http.StatusInternalServerError, "gagal mengembalikan saldo"
		}
	}
	if auditAction != "" {
		if err := auditTx(ctx, tx, actorID, auditAction, orderID, status, "CANCELLED", auditIP); err != nil {
			return http.StatusInternalServerError, "gagal mencatat audit"
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return http.StatusInternalServerError, "gagal menyimpan pembatalan"
	}
	return 0, ""
}
