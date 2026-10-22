package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ai-sell/gateway/internal/provider"
	"ai-sell/gateway/internal/secret"
)

// Panel admin. Satu berkas karena semuanya berbentuk sama: baca satu tabel,
// satu tindakan tulis — memecahnya jadi delapan berkas hanya menambah impor.

// roles adalah daftar TERTUTUP. Peran tidak pernah dibaca bebas dari body:
// satu PATCH bisa menulis nilai apa pun ke kolom hak akses tertinggi.
var roles = map[string]bool{"USER": true, "SELLER": true, "ADMIN": true, "SUPER_ADMIN": true}

// OrderComplete: tandai pesanan selesai → escrow cair ke seller.
func OrderComplete(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")
		res, reason, fine := releaseEscrowAudited(r.Context(), db, id, uid, "ORDER_COMPLETE", ipOf(r))
		if !fine {
			fail(w, http.StatusConflict, reason)
			return
		}
		ok(w, map[string]any{"ok": true, "seller_payout": res.SellerPayout, "fee": res.FeeAmount})
	}
}

// AdminOrders: GET /api/admin/orders.
func AdminOrders(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		// Lazy auto-cancel saat admin membuka daftar order.
		ReconcileOverdueOrders(r.Context(), db)
		rows, err := db.Query(r.Context(),
			`SELECT o.id, o.qty, o.subtotal, o.seller_payout, o.fee_amount, o.status, o.created_at,
			        COALESCE(o.completed_at, o.created_at), COALESCE(u.email,''), p.title, COALESCE(s.name,'')
			   FROM market_order o
			   JOIN market_product p ON p.id = o.product_id
			   LEFT JOIN market_store s ON s.id = o.store_id
			   LEFT JOIN "user" u ON u.id = o.buyer_id
			  WHERE ($1 = '' OR o.status = $1)
			  ORDER BY o.created_at DESC LIMIT 200`, status)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type o struct {
			ID        string    `json:"id"`
			Qty       int       `json:"qty"`
			Subtotal  float64   `json:"subtotal"`
			Payout    float64   `json:"seller_payout"`
			Fee       float64   `json:"fee_amount"`
			Status    string    `json:"status"`
			CreatedAt time.Time `json:"created_at"`
			ClosedAt  time.Time `json:"updated_at"`
			// Halaman /admin/orders membaca o.buyer_email.
			Buyer     string `json:"buyer_email"`
			Title     string `json:"product_title"`
			StoreName string `json:"store_name"`
		}
		out := []o{}
		for rows.Next() {
			var x o
			if err := rows.Scan(&x.ID, &x.Qty, &x.Subtotal, &x.Payout, &x.Fee, &x.Status, &x.CreatedAt,
				&x.ClosedAt, &x.Buyer, &x.Title, &x.StoreName); err != nil {
				continue
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in AdminOrders: %v", err)
		}
		ok(w, map[string]any{"orders": out})
	}
}

// AdminRefundOrder: POST /api/admin/orders/{id}/refund — uang pembeli kembali,
// stok dikembalikan. Hanya untuk order yang escrow-nya belum cair.
func AdminRefundOrder(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")
		code, reason := cancelOrder(r.Context(), db, id, uid, true, "Refund admin", "ORDER_REFUND", ipOf(r))
		if code != 0 {
			fail(w, code, reason)
			return
		}
		ok(w, map[string]any{"ok": true, "status": "CANCELLED"})
	}
}

// AdminPatchStore: PATCH /api/admin/stores/{id} — aktivasi/bekukan toko.
// Skema: is_active default false "diaktifkan admin setelah review"; tanpa
// endpoint ini toko tidak pernah bisa diaktifkan dan checkout menolak semua
// tokonya (guard anti toko-nonaktif di checkoutOrder).
func AdminPatchStore(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			IsActive *bool `json:"is_active"`
		}
		if err := decode(r, &in); err != nil || in.IsActive == nil {
			fail(w, http.StatusBadRequest, "is_active wajib boolean")
			return
		}
		id := r.PathValue("id")
		tag, err := db.Exec(r.Context(),
			`UPDATE market_store SET is_active=$1, updated_at=now() WHERE id=$2`, *in.IsActive, id)
		if err != nil || tag.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "toko tidak ditemukan")
			return
		}
		aksi := "bekukan"
		if *in.IsActive {
			aksi = "aktifkan"
		}
		audit(r.Context(), db, uidFromCtx(r.Context()), "STORE_MODERATE", id, "review", aksi, ipOf(r))
		ok(w, map[string]any{"ok": true, "is_active": *in.IsActive})
	}
}

// AdminProducts: GET /api/admin/products. Filter: ?aktif=true|false.
func AdminProducts(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		aktif := r.URL.Query().Get("aktif")
		rows, err := db.Query(r.Context(),
			`SELECT p.id, p.title, p.price, p.stock, p.sales_count, p.is_active, p.is_boosted, p.pin_order,
			        COALESCE(s.name,''), COALESCE(c.name,''), p.created_at
			   FROM market_product p
			   LEFT JOIN market_store s ON s.id = p.store_id
			   LEFT JOIN market_category c ON c.id = p.category_id
			  WHERE ($1 = '' OR p.is_active = ($1 = 'true'))
			  ORDER BY p.created_at DESC LIMIT 200`, aktif)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type p struct {
			ID         string  `json:"id"`
			Title      string  `json:"title"`
			Price      float64 `json:"price"`
			Stock      int     `json:"stock"`
			SalesCount int     `json:"sales_count"`
			IsActive   bool    `json:"is_active"`
			IsBoosted  bool    `json:"is_boosted"`
			PinOrder   *int    `json:"pin_order,omitempty"`
			Store      string  `json:"store_name"`
			// Nama yang dibaca halaman /admin/products (p.store berupa string).
			Store2    string    `json:"store"`
			Category  string    `json:"category_name,omitempty"`
			CreatedAt time.Time `json:"created_at"`
		}
		out := []p{}
		for rows.Next() {
			var x p
			if err := rows.Scan(&x.ID, &x.Title, &x.Price, &x.Stock, &x.SalesCount, &x.IsActive, &x.IsBoosted, &x.PinOrder,
				&x.Store, &x.Category, &x.CreatedAt); err != nil {
				continue
			}
			x.Store2 = x.Store
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in AdminProducts: %v", err)
		}
		ok(w, map[string]any{"products": out})
	}
}

// AdminPatchProduct: PATCH /api/admin/products {id,...} — id boleh dari body
// atau dari path (/api/admin/products/{id}, itu yang dipanggil /admin/products).
func AdminPatchProduct(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			ID        string `json:"id"`
			IsActive  *bool  `json:"is_active"`
			IsBoosted *bool  `json:"is_boosted"`
			PinOrder  *int   `json:"pin_order"`
			ClearPin  bool   `json:"clear_pin_order"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		if in.ID == "" {
			in.ID = r.PathValue("id")
		}
		if in.ID == "" {
			fail(w, http.StatusBadRequest, "id wajib")
			return
		}
		var before struct {
			IsActive  bool `json:"is_active"`
			IsBoosted bool `json:"is_boosted"`
			PinOrder  *int `json:"pin_order"`
		}
		if err := db.QueryRow(r.Context(),
			`SELECT is_active, is_boosted, pin_order FROM market_product WHERE id=$1`, in.ID).
			Scan(&before.IsActive, &before.IsBoosted, &before.PinOrder); err != nil {
			fail(w, http.StatusNotFound, "produk tidak ditemukan")
			return
		}
		var err error
		if in.ClearPin {
			_, err = db.Exec(r.Context(),
				`UPDATE market_product SET is_active=COALESCE($1,is_active), is_boosted=COALESCE($2,is_boosted),
				        pin_order=NULL, updated_at=now() WHERE id=$3`,
				in.IsActive, in.IsBoosted, in.ID)
		} else {
			_, err = db.Exec(r.Context(),
				`UPDATE market_product SET is_active=COALESCE($1,is_active), is_boosted=COALESCE($2,is_boosted),
				        pin_order=COALESCE($3,pin_order), updated_at=now() WHERE id=$4`,
				in.IsActive, in.IsBoosted, in.PinOrder, in.ID)
		}
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal mengubah produk")
			return
		}
		audit(r.Context(), db, uid, "PRODUCT_MODERATE", in.ID, before,
			map[string]any{"is_active": in.IsActive, "is_boosted": in.IsBoosted, "pin_order": in.PinOrder}, ipOf(r))
		ok(w, map[string]any{"ok": true})
	}
}

// AdminWallet: GET /api/admin/wallet?status=PENDING — uang yang tertahan,
// menunggu keputusan admin.
func AdminWallet(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		if status == "" {
			status = "PENDING"
		}
		rows, err := db.Query(r.Context(),
			`SELECT w.id, w.amount, w.status, COALESCE(w.payment_type,''), COALESCE(w.payout_ref,''),
			        COALESCE(w.destination,''), w.fee_amount, w.net_amount, w.created_at, COALESCE(u.email,''), COALESCE(u.name,'')
			   FROM withdrawal w JOIN "user" u ON u.id = w.user_id
			  WHERE w.status=$1 ORDER BY w.created_at LIMIT 100`, status)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type wd struct {
			ID          string  `json:"id"`
			Amount      float64 `json:"amount"`
			Status      string  `json:"status"`
			PaymentType string  `json:"payment_type,omitempty"`
			PayoutRef   string  `json:"payout_ref,omitempty"`
			// Tujuan pencairan: admin yang memutuskan approve/reject harus tahu
			// uangnya dikirim ke mana (nama bank + nomor rekening dari user).
			Destination string    `json:"destination,omitempty"`
			BankAccount string    `json:"bank_account,omitempty"`
			FeeAmount   float64   `json:"fee_amount"`
			NetAmount   float64   `json:"net_amount"`
			CreatedAt   time.Time `json:"created_at"`
			Email       string    `json:"user_email"`
			Name        string    `json:"name"`
		}
		var withdrawals []wd
		for rows.Next() {
			var x wd
			if err := rows.Scan(&x.ID, &x.Amount, &x.Status, &x.PaymentType, &x.PayoutRef, &x.Destination, &x.FeeAmount, &x.NetAmount, &x.CreatedAt, &x.Email, &x.Name); err != nil {
				continue
			}
			x.BankAccount = x.Destination
			withdrawals = append(withdrawals, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in AdminWallet: %v", err)
		}
		rows.Close()

		// Top-up manual menunggu verifikasi transfer.
		trs, err := db.Query(r.Context(),
			`SELECT t.id, t.amount, t.status, t.description, COALESCE(t.reference_id,''), COALESCE(t.invoice_id,''), COALESCE(t.proof_url,''), t.created_at,
			        COALESCE(u.email,''), COALESCE(u.name,'')
			   FROM credit_transaction t JOIN "user" u ON u.id = t.user_id
			  WHERE t.type='TOPUP' AND t.status=$1 ORDER BY t.created_at LIMIT 100`, status)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer trs.Close()
		type tr struct {
			ID          string  `json:"id"`
			Amount      float64 `json:"amount"`
			Status      string  `json:"status"`
			Description string  `json:"description"`
			// Metode bayar yang dipilih user saat minta top-up (reference_id).
			Method    string    `json:"method,omitempty"`
			Reference string    `json:"payment_reference,omitempty"`
			ProofURL  string    `json:"proof_url,omitempty"`
			CreatedAt time.Time `json:"created_at"`
			Email     string    `json:"user_email"`
			Name      string    `json:"name"`
		}
		var topups []tr
		for trs.Next() {
			var x tr
			if err := trs.Scan(&x.ID, &x.Amount, &x.Status, &x.Description, &x.Method, &x.Reference, &x.ProofURL, &x.CreatedAt, &x.Email, &x.Name); err != nil {
				continue
			}
			topups = append(topups, x)
		}
		if err := trs.Err(); err != nil {
			log.Printf("[WARN] rows error in AdminWallet: %v", err)
		}
		ok(w, map[string]any{"withdrawals": withdrawals, "topups": topups})
	}
}

// DecideWithdrawal: POST /api/admin/withdrawals/{id}/{approve|reject}.
// APPROVED = uang sudah dipotong saat pengajuan; REJECTED = dikembalikan.
// Syarat status ikut di dalam UPDATE: dua admin menekan bersamaan hanya
// menghasilkan satu perubahan.
func DecideWithdrawal(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")
		action := r.PathValue("action")
		target := map[string]string{"approve": "APPROVED", "reject": "REJECTED"}[action]
		if target == "" {
			fail(w, http.StatusBadRequest, "aksi harus approve atau reject")
			return
		}

		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		defer tx.Rollback(r.Context())

		moved, err := tx.Exec(r.Context(),
			`UPDATE withdrawal SET status=$1, updated_at=now() WHERE id=$2 AND status='PENDING'`, target, id)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		if moved.RowsAffected() == 0 {
			fail(w, http.StatusConflict, "penarikan sudah diproses")
			return
		}
		var userID string
		var amount float64
		if err := tx.QueryRow(r.Context(),
			`SELECT user_id, amount FROM withdrawal WHERE id=$1`, id).Scan(&userID, &amount); err != nil {
			fail(w, http.StatusNotFound, "penarikan tidak ditemukan")
			return
		}
		if target == "REJECTED" {
			if _, err := tx.Exec(r.Context(),
				`UPDATE "user" SET wallet_balance = wallet_balance + $1 WHERE id=$2`, amount, userID); err != nil {
				fail(w, http.StatusInternalServerError, "gagal mengembalikan saldo")
				return
			}
		}
		// Baris buku besar ikut statusnya, supaya total debit di buku besar tetap
		// cocok dengan saldo; kalau tidak, audit saldo vs ledger akan selisih.
		desc := "Penarikan disetujui"
		ledger := "COMPLETED"
		if target == "REJECTED" {
			desc, ledger = "Penarikan ditolak — saldo dikembalikan", "REFUNDED"
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE credit_transaction SET status=$1, description=$2 WHERE invoice_id=$3 AND type='WITHDRAWAL'`,
			ledger, desc, id); err != nil {
			fail(w, http.StatusInternalServerError, "gagal memperbarui buku besar")
			return
		}
		if err := auditTx(r.Context(), tx, uid, "WITHDRAWAL_"+target, id, "PENDING", target, ipOf(r)); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mencatat audit")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan keputusan")
			return
		}
		ok(w, map[string]any{"ok": true, "status": target})
	}
}

// SettleTopup: POST /api/admin/topup/{id}/complete|fail. Saldo + akumulasi naik
// HANYA di sini dan hanya kalau transaksinya masih PENDING (idempoten).
func SettleTopup(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")
		action := map[string]string{"complete": "COMPLETED", "fail": "FAILED", "reject": "REJECTED"}[r.PathValue("action")]
		if action == "" {
			fail(w, http.StatusBadRequest, "aksi harus complete atau fail")
			return
		}

		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		defer tx.Rollback(r.Context())

		moved, err := tx.Exec(r.Context(),
			`UPDATE credit_transaction SET status=$1 WHERE id=$2 AND type='TOPUP' AND status='PENDING'`, action, id)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		if moved.RowsAffected() == 0 {
			fail(w, http.StatusConflict, "top-up sudah diproses")
			return
		}
		var userID string
		var amount float64
		if err := tx.QueryRow(r.Context(),
			`SELECT user_id, amount FROM credit_transaction WHERE id=$1`, id).Scan(&userID, &amount); err != nil {
			fail(w, http.StatusNotFound, "transaksi tidak ditemukan")
			return
		}
		if action == "COMPLETED" {
			if _, err := tx.Exec(r.Context(),
				`UPDATE "user" SET wallet_balance = wallet_balance + $1 WHERE id=$2`, amount, userID); err != nil {
				fail(w, http.StatusInternalServerError, "gagal menambah saldo")
				return
			}
			// Akumulasi milestone naik bersamaan dengan uang yang benar-benar
			// masuk — bukan saat pengajuan.
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO user_topup_accumulation (user_id, total_topup) VALUES ($1,$2)
				 ON CONFLICT (user_id) DO UPDATE SET total_topup = user_topup_accumulation.total_topup + EXCLUDED.total_topup,
				   updated_at=now()`, userID, amount); err != nil {
				log.Printf("[WARN] akumulasi topup gagal: %v", err)
			}
			// Referral bonus dan token commission diproses pada pembelian paket token.
			// Top-up Wallet sendiri tidak membayar referral dalam Rupiah.			// Bonus undangan (kedua sisi) lunas di topup PERTAMA invitee —
			// bukan saat daftar (anti-farming akun buangan).
			payDeferredReferralBonus(r.Context(), tx, userID)
			if err := enqueueEmailOutbox(r.Context(), tx, "manual:"+id, emailEventManual, userID, id); err != nil {
				fail(w, http.StatusInternalServerError, "gagal menjadwalkan email top-up")
				return
			}
		}
		if err := auditTx(r.Context(), tx, uid, "TOPUP_"+action, id, "PENDING", action, ipOf(r)); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mencatat audit")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan keputusan")
			return
		}
		credited := 0.0
		if action == "COMPLETED" {
			credited = amount
		}
		ok(w, map[string]any{"ok": true, "status": action, "credited": credited})
	}
}

// AdminUptimeGet: GET /api/admin/uptime?hours=24.
func AdminUptimeGet(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hours := 24
		if v, err := strconv.Atoi(r.URL.Query().Get("hours")); err == nil && v > 0 && v <= 168 {
			hours = v
		}
		rows, err := db.Query(r.Context(),
			`SELECT m.id, m.name, m.kind, m.url, m.is_active, m.interval_sec,
			        COALESCE(h.status,'unknown'), h.latency_ms, h.checked_at
			   FROM uptime_monitor m
			   LEFT JOIN LATERAL (
			     SELECT status, latency_ms, checked_at FROM uptime_heartbeat
			      WHERE monitor_id = m.id ORDER BY checked_at DESC LIMIT 1) h ON true
			  ORDER BY m.name`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type mon struct {
			ID          string     `json:"id"`
			Name        string     `json:"name"`
			Kind        string     `json:"kind"`
			URL         string     `json:"url"`
			IsActive    bool       `json:"is_active"`
			IntervalSec int        `json:"interval_sec"`
			Status      string     `json:"status"`
			LatencyMs   *int       `json:"latency_ms,omitempty"`
			CheckedAt   *time.Time `json:"checked_at,omitempty"`
		}
		out := []mon{}
		for rows.Next() {
			var x mon
			if err := rows.Scan(&x.ID, &x.Name, &x.Kind, &x.URL, &x.IsActive, &x.IntervalSec, &x.Status, &x.LatencyMs, &x.CheckedAt); err != nil {
				continue
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in AdminUptimeGet: %v", err)
		}
		var count int
		_ = db.QueryRow(r.Context(), `SELECT COUNT(*) FROM uptime_monitor`).Scan(&count)
		ok(w, map[string]any{"hours": hours, "monitors": out, "seeded": count > 0})
	}
}

// AdminUptimePost: POST /api/admin/uptime {action: run|add|toggle|seed}.
//
// ponytail: hanya HTTP yang benar-benar dijalankan. Monitor kind TCP/PRISMA
// tersimpan dan tampil, tapi tidak diprobe — menambahkan prober kedua duplikat.
func AdminUptimePost(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Action      string `json:"action"`
			Name        string `json:"name"`
			URL         string `json:"url"`
			Kind        string `json:"kind"`
			ID          string `json:"id"`
			IsActive    *bool  `json:"is_active"`
			IntervalSec int    `json:"interval_sec"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		if in.Action == "" {
			in.Action = "run"
		}

		switch in.Action {
		case "seed":
			created := 0
			for _, d := range []struct{ name, u string }{
				{"Gateway (healthz)", "http://127.0.0.1:8080/healthz"},
				{"Marketplace API", "http://127.0.0.1:8080/api/market/products"},
			} {
				res, err := db.Exec(r.Context(),
					`INSERT INTO uptime_monitor (name, kind, url) VALUES ($1,'HTTP',$2)
					 ON CONFLICT DO NOTHING`, d.name, d.u)
				if err == nil {
					if n := res.RowsAffected(); n > 0 {
						created += int(n)
					}
				}
			}
			ok(w, map[string]any{"ok": true, "created": created})
			return

		case "add":
			name := strings.TrimSpace(in.Name)
			target := strings.TrimSpace(in.URL)
			if name == "" || target == "" {
				fail(w, http.StatusBadRequest, "name + url wajib")
				return
			}
			// URL diperiksa di sini, bukan di UI: monitor menerima alamat apa pun
			// dan akan memanggilnya berulang kali dari server.
			// Host non-publik ditolak karena gateway jadi proksi probe ke
			// jaringan internal (SSRF). Dev yang memang memantau gateway/db
			// sendiri membukanya lewat UPTIME_ALLOW_INTERNAL=true.
			allowInternal := os.Getenv("UPTIME_ALLOW_INTERNAL") == "true"
			if _, err := url.ParseRequestURI(target); err != nil || (!strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://")) {
				fail(w, http.StatusBadRequest, "url harus http(s)://")
				return
			}
			if !allowInternal {
				if h := urlHost(target); h != "" && !uptimeHostAllowed(h, false) {
					fail(w, http.StatusBadRequest, "url harus alamat publik (set UPTIME_ALLOW_INTERNAL=true untuk host internal)")
					return
				}
			}
			kind := in.Kind
			if kind != "HTTP" && kind != "TCP" && kind != "PRISMA" && kind != "PROCESS" {
				kind = "HTTP"
			}
			interval := in.IntervalSec
			if interval < 30 {
				interval = 60
			}
			var id string
			if err := db.QueryRow(r.Context(),
				`INSERT INTO uptime_monitor (name, kind, url, interval_sec) VALUES ($1,$2,$3,$4) RETURNING id`,
				name, kind, target, interval).Scan(&id); err != nil {
				fail(w, http.StatusInternalServerError, "gagal menambah monitor")
				return
			}
			ok(w, map[string]any{"ok": true, "id": id})
			return

		case "toggle":
			if in.ID == "" {
				fail(w, http.StatusBadRequest, "id wajib")
				return
			}
			active := in.IsActive != nil && *in.IsActive
			res, err := db.Exec(r.Context(), `UPDATE uptime_monitor SET is_active=$1 WHERE id=$2`, active, in.ID)
			if err != nil || res.RowsAffected() == 0 {
				fail(w, http.StatusNotFound, "monitor tidak ditemukan")
				return
			}
			ok(w, map[string]any{"ok": true, "is_active": active})
			return
		}

		// action=run: periksa monitor HTTP aktif dan simpan heartbeat.
		checked := runUptimeChecks(r.Context(), db)
		ok(w, map[string]any{"ok": true, "checked": len(checked), "results": checked})
	}
}

type uptimeResult struct {
	MonitorID string    `json:"monitor_id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	LatencyMs int       `json:"latency_ms"`
	CheckedAt time.Time `json:"checked_at"`
}

// uptimeHostAllowed: tolak target yang bukan alamat publik supaya gateway tidak
// jadi proksi probe ke jaringan internal (SSRF). Parser URL-nya net/url, sama
// dengan yang akan dipakai dialer. Nama layanan Docker ("gateway") bukan alamat
// publik. Dipakai dua kali: saat menyimpan dan saat mengikuti redirect.
func uptimeHostAllowed(host string, allowInternal bool) bool {
	if allowInternal {
		return true
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" || host == "localhost" {
		return false
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified())
	}
	// Hostname tanpa titik (single-label) tidak pernah jadi alamat publik yang
	// sah — itulah cara nama layanan Docker ("gateway", "db") terlihat.
	if !strings.Contains(host, ".") {
		return false
	}
	// Metadata cloud GCP/EGH memakai nama, bukan IP.
	if host == "metadata.google.internal" {
		return false
	}
	// ponytail: hostname yang resolve ke privat tetap lolos di sini (tidak ada
	// DNS lookup saat validasi). Kalau perlu ketat, resolve lalu periksa IP-nya.
	return true
}

// safeDialControl: pemeriksaan M-2 pada lapisan paling akhir — saat koneksi
// benar-benar dibuka, memakai alamat IP hasil resolusi DNS. Tanpa ini, hostname
// yang tampak publik tapi resolve ke 127.0.0.1 lolos dari pemeriksaan nama.
func safeDialControl(allowInternal bool) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, _ syscall.RawConn) error {
		if allowInternal {
			return nil
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			host = address
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("alamat tidak bisa dipastikan: %s", address)
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("alamat internal ditolak: %s", ip)
		}
		return nil
	}
}

func urlHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func runUptimeChecks(ctx context.Context, db DB) []uptimeResult {
	rows, err := db.Query(ctx,
		`SELECT id, name, url, timeout_ms, slow_ms FROM uptime_monitor WHERE is_active AND kind='HTTP' ORDER BY name`)
	if err != nil {
		return nil
	}
	type target struct {
		id, name, url string
		timeout, slow int
	}
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.name, &t.url, &t.timeout, &t.slow); err != nil {
			continue
		}
		targets = append(targets, t)
	}
	rows.Close()

	// Redirect tidak boleh dipakai menembus pemeriksaan host di atas, dan
	// dial-nya diperiksa setelah DNS (host yang resolve ke IP privat ditolak).
	tr := &http.Transport{}
	tr.DialContext = (&net.Dialer{
		Timeout:   5 * time.Second,
		Control:   safeDialControl(os.Getenv("UPTIME_ALLOW_INTERNAL") == "true"),
		KeepAlive: 30 * time.Second,
	}).DialContext
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			if !uptimeHostAllowed(req.URL.Hostname(), os.Getenv("UPTIME_ALLOW_INTERNAL") == "true") {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	out := []uptimeResult{}
	for _, t := range targets {
		if t.timeout <= 0 {
			t.timeout = 8000
		}
		if t.slow <= 0 {
			t.slow = 3000
		}
		start := time.Now()
		status, code := "up", 0
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
		if err != nil {
			status = "down"
		} else {
			resp, err := client.Do(req)
			if err != nil {
				status = "down"
			} else {
				code = resp.StatusCode
				resp.Body.Close()
				// 401/404/405 = endpoint hidup tapi tidak punya jalur periksa.
				// Menyimpulkan "mati" dari balasan itu melaporkan provider sehat
				// sebagai gangguan.
				if code >= 500 {
					status = "down"
				}
			}
		}
		latency := int(time.Since(start).Milliseconds())
		if status == "up" && latency > t.slow {
			status = "slow"
		}
		var checkedAt time.Time
		if err := db.QueryRow(ctx,
			`INSERT INTO uptime_heartbeat (monitor_id, status, latency_ms, status_code) VALUES ($1,$2,$3,$4)
			 RETURNING checked_at`, t.id, status, latency, code).Scan(&checkedAt); err != nil {
			checkedAt = time.Now()
		}
		out = append(out, uptimeResult{MonitorID: t.id, Name: t.name, Status: status, LatencyMs: latency, CheckedAt: checkedAt})
	}
	// Riwayat dipangkas: tanpa ini tabel tumbuh tanpa batas, sementara ringkasan
	// yang ditampilkan hanya 24 jam.
	_, _ = db.Exec(ctx, `DELETE FROM uptime_heartbeat WHERE checked_at < now() - interval '30 days'`)
	_, _ = db.Exec(ctx, `DELETE FROM model_health_log WHERE checked_at < now() - interval '30 days'`)
	return out
}

// AdminHealth: GET /api/admin/health — periksa jalur MODEL + catat hasilnya.
func AdminHealth(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(),
			`SELECT m.id, m.name, MIN(p.provider) FROM ai_model m
			   JOIN model_pricing p ON p.model_id = m.id AND p.is_active
			  WHERE m.is_active GROUP BY m.id, m.name ORDER BY m.name`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		type model struct{ id, name, provider string }
		var models []model
		for rows.Next() {
			var m model
			if err := rows.Scan(&m.id, &m.name, &m.provider); err != nil {
				continue
			}
			models = append(models, m)
		}
		rows.Close()

		type logRow struct {
			Provider   string `json:"provider"`
			Model      string `json:"model"`
			Status     string `json:"status"`
			Latency    int    `json:"latency"`
			StatusCode *int   `json:"status_code,omitempty"`
			Message    string `json:"message,omitempty"`
		}
		var logs []logRow
		client := &http.Client{Timeout: 8 * time.Second}

		for _, m := range models {
			var pid, baseURL, stored string
			err := db.QueryRow(r.Context(),
				`SELECT id, base_url, api_key FROM provider WHERE name=$1`, m.provider).Scan(&pid, &baseURL, &stored)
			if err != nil {
				logs = append(logs, logRow{Provider: m.provider, Model: m.name, Status: "unknown",
					Message: "provider belum terdaftar"})
				continue
			}
			key, decErr := secret.Decrypt(stored)
			hasKey := stored != "" && decErr == nil

			start := time.Now()
			status, message := "ok", ""
			var code *int
			req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, strings.TrimRight(baseURL, "/")+"/models", nil)
			if err != nil {
				status, message = "down", "base URL tidak sah"
			} else {
				if hasKey {
					req.Header.Set("Authorization", "Bearer "+key)
				}
				resp, err := client.Do(req)
				if err != nil {
					status, message = "down", "tidak bisa dihubungi"
				} else {
					c := resp.StatusCode
					code = &c
					resp.Body.Close()
					switch {
					case c >= 500:
						status, message = "down", "provider membalas "+itoa(c)
					case c == 401 || c == 403:
						status, message = "ok", "kunci ditolak provider"
					case c == 404 || c == 405:
						status, message = "ok", "endpoint hidup, jalur pemeriksaan tidak tersedia"
					}
				}
			}
			latency := int(time.Since(start).Milliseconds())
			if status == "ok" && latency > 3000 {
				status = "slow"
			}
			if _, err := db.Exec(r.Context(),
				`UPDATE provider SET is_healthy=$1, last_checked=now(), last_check_status=$2, last_latency_ms=$3, updated_at=now() WHERE id=$4`,
				status != "down", status, latency, pid); err != nil {
				log.Printf("[WARN] update kesehatan provider gagal: %v", err)
			}
			if _, err := db.Exec(r.Context(),
				`INSERT INTO model_health_log (model_id, provider_id, status, latency_ms, status_code)
				 VALUES ($1,$2,$3,$4,$5)`, m.id, pid, status, latency, code); err != nil {
				log.Printf("[WARN] catat health log gagal: %v", err)
			}
			logs = append(logs, logRow{Provider: m.provider, Model: m.name, Status: status, Latency: latency, StatusCode: code, Message: message})
		}
		ok(w, map[string]any{"ok": true, "checked": len(logs), "logs": logs})
	}
}

// AdminRequestLogs: GET /api/admin/requests — metadata-only /v1 request ledger.
// Supports ?limit=, ?offset=, ?status= filter. Pagination via LIMIT/OFFSET.
func AdminRequestLogs(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 100
		if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 500 {
			limit = v
		}
		offset := 0
		if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
			offset = v
		}
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		q := `SELECT id, COALESCE(user_id::text,''), COALESCE(key_id::text,''), model, provider, input_tokens, output_tokens, cached_tokens, cost_tokens, latency_ms, status, error_code, created_at FROM api_request_log`
		args := []any{}
		where := ""
		if status != "" {
			where = " WHERE status=$1"
			args = append(args, status)
		}
		q += where + ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
		args = append(args, limit, offset)
		rows, err := db.Query(r.Context(), q, args...)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type row struct {
			ID, UserID, KeyID, Model, Provider   string
			Input, Output, Cached, Cost, Latency int64
			Status, Error                        string
			Created                              time.Time
		}
		out := []row{}
		for rows.Next() {
			var x row
			if rows.Scan(&x.ID, &x.UserID, &x.KeyID, &x.Model, &x.Provider, &x.Input, &x.Output, &x.Cached, &x.Cost, &x.Latency, &x.Status, &x.Error, &x.Created) == nil {
				out = append(out, x)
			}
		}
		ok(w, map[string]any{"logs": out, "limit": limit, "offset": offset, "status": status})
	}
}

// Query SQL dipisah jadi konstanta supaya bisa dijaga test tanpa DB.
const (
	adminFraudListSQL = `SELECT f.id, f.user_id, COALESCE(u.email,''), f.kind, f.detail, COALESCE(f.ip,''),` +
		` f.is_reviewed, f.is_actioned, f.created_at` +
		` FROM fraud_event f LEFT JOIN "user" u ON u.id = f.user_id` +
		` ORDER BY f.created_at DESC LIMIT 200`
	adminFraudReviewSQL = `UPDATE fraud_event SET is_reviewed=true WHERE id=$1 AND NOT is_reviewed`
)

// AdminFraudEvents: GET /api/admin/fraud — daftar event fraud terbaru.
// Read-only; admin sudah bisa menindak lewat /admin/users (ban/unban).
func AdminFraudEvents(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(), adminFraudListSQL)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type row struct {
			ID         string    `json:"id"`
			UserID     string    `json:"user_id"`
			Email      string    `json:"email"`
			Kind       string    `json:"kind"`
			Detail     string    `json:"detail"`
			IP         string    `json:"ip"`
			IsReviewed bool      `json:"is_reviewed"`
			IsActioned bool      `json:"is_actioned"`
			CreatedAt  time.Time `json:"created_at"`
		}
		out := []row{}
		for rows.Next() {
			var x row
			if rows.Scan(&x.ID, &x.UserID, &x.Email, &x.Kind, &x.Detail, &x.IP, &x.IsReviewed, &x.IsActioned, &x.CreatedAt) == nil {
				out = append(out, x)
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in AdminFraudEvents: %v", err)
		}
		ok(w, map[string]any{"events": out})
	}
}

// AdminFraudReview: POST /api/admin/fraud/{id}/review — tandai event sudah
// ditinjau. Tanpa aksi otomatis: keputusan ban tetap lewat /admin/users.
func AdminFraudReview(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		tag, err := db.Exec(r.Context(), adminFraudReviewSQL, id)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memperbarui event fraud")
			return
		}
		ok(w, map[string]any{"ok": true, "id": id, "changed": tag.RowsAffected()})
	}
}

// AdminAudit: GET /api/admin/audit — jejak perubahan penting.
func AdminAudit(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")
		rows, err := db.Query(r.Context(),
			`SELECT a.id, COALESCE(u.email,''), a.action, COALESCE(a.target,''), COALESCE(a.before,''),
			        COALESCE(a.after,''), COALESCE(a.ip,''), a.created_at
			   FROM audit_log a LEFT JOIN "user" u ON u.id = a.user_id
			  WHERE ($1 = '' OR a.action = $1) ORDER BY a.created_at DESC LIMIT 200`, action)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type a struct {
			ID        string    `json:"id"`
			Email     string    `json:"email"`
			Action    string    `json:"action"`
			Target    string    `json:"target"`
			Before    string    `json:"before"`
			After     string    `json:"after"`
			IP        string    `json:"ip"`
			CreatedAt time.Time `json:"created_at"`
		}
		out := []a{}
		for rows.Next() {
			var x a
			if err := rows.Scan(&x.ID, &x.Email, &x.Action, &x.Target, &x.Before, &x.After, &x.IP, &x.CreatedAt); err != nil {
				continue
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in AdminAudit: %v", err)
		}
		ok(w, map[string]any{"audit": out})
	}
}

// AdminBanUser: POST /api/admin/users/{id}/ban {reason}.
// Menyetel is_banned=true, mencatat alasan + timestamp, menghapus semua sesi
// agar token JWT/sesi yang sudah diterbitkan langsung tidak berlaku lagi.
func AdminBanUser(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		target := r.PathValue("id")
		var in struct {
			Reason string `json:"reason"`
		}
		_ = decode(r, &in)
		reason := strings.TrimSpace(in.Reason)
		if len(reason) < 1 || len(reason) > 500 {
			fail(w, http.StatusBadRequest, "reason wajib 1–500 karakter")
			return
		}
		// Admin terakhir tidak boleh diblokir: platform tanpa admin tidak
		// bisa pulih dari dalam aplikasi.
		var role string
		if err := db.QueryRow(r.Context(), `SELECT role FROM "user" WHERE id=$1`, target).Scan(&role); err != nil {
			fail(w, http.StatusNotFound, "pengguna tidak ditemukan")
			return
		}
		if isAdminRole(role) {
			var admins int
			if err := db.QueryRow(r.Context(),
				`SELECT COUNT(*) FROM "user" WHERE role IN ('ADMIN','SUPER_ADMIN') AND id != $1`, target).Scan(&admins); err != nil {
				fail(w, http.StatusInternalServerError, "gagal memeriksa admin")
				return
			}
			if admins < 1 {
				fail(w, http.StatusBadRequest, "ini admin terakhir — angkat admin lain dulu")
				return
			}
		}
		if _, err := db.Exec(r.Context(),
			`UPDATE "user" SET is_banned=true, banned_reason=$1, banned_at=now(), updated_at=now() WHERE id=$2`,
			reason, target); err != nil {
			fail(w, http.StatusInternalServerError, "gagal memblokir akun")
			return
		}
		// Cabut semua sesi: semua JWT/sesi yang sudah diterbitkan langsung
		// tidak berlaku lagi (RequireAuth memeriksa user_session).
		if _, err := db.Exec(r.Context(), `DELETE FROM user_session WHERE user_id=$1`, target); err != nil {
			log.Printf("[WARN] hapus sesi user diblokir gagal: id=%s err=%v", target, err)
		}
		// Notifikasi ke user yang diblokir: terjimpan di tabel notifications
		// dan akan muncul saat mereka mencoba login kembali.
		genSystemNotification(r.Context(), db, target, "Akun Diblokir", reason)
		audit(r.Context(), db, uid, "USER_BAN", target, false, true, ipOf(r))
		ok(w, map[string]any{"ok": true, "id": target, "banned": true})
	}
}

// AdminUnbanUser: POST /api/admin/users/{id}/unban.
// Mengembalikan akses login/API: is_banned=false, bersihkan banned_reason/at.
func AdminUnbanUser(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		target := r.PathValue("id")
		var banned bool
		if err := db.QueryRow(r.Context(), `SELECT is_banned FROM "user" WHERE id=$1`, target).Scan(&banned); err != nil {
			fail(w, http.StatusNotFound, "pengguna tidak ditemukan")
			return
		}
		if !banned {
			ok(w, map[string]any{"ok": true, "unbanned": true, "already": true})
			return
		}
		if _, err := db.Exec(r.Context(),
			`UPDATE "user" SET is_banned=false, banned_reason=NULL, banned_at=NULL, updated_at=now() WHERE id=$1`, target); err != nil {
			fail(w, http.StatusInternalServerError, "gagal melepas blokir akun")
			return
		}
		genSystemNotification(r.Context(), db, target, "Akun Diaktifkan Kembali", "Blokir akun Anda telah dicabut oleh admin.")
		audit(r.Context(), db, uid, "USER_UNBAN", target, true, false, ipOf(r))
		ok(w, map[string]any{"ok": true, "id": target, "unbanned": true})
	}
}

// AdminDeleteUser: DELETE /api/admin/users/{id}.
// Hapus akun — semua data terkait ikut terhapus lewat ON DELETE CASCADE.
// Admin terakhir tidak boleh dihapus.
func AdminDeleteUser(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		target := r.PathValue("id")
		var role string
		if err := db.QueryRow(r.Context(), `SELECT role FROM "user" WHERE id=$1`, target).Scan(&role); err != nil {
			fail(w, http.StatusNotFound, "pengguna tidak ditemukan")
			return
		}
		if isAdminRole(role) {
			var admins int
			if err := db.QueryRow(r.Context(),
				`SELECT COUNT(*) FROM "user" WHERE role IN ('ADMIN','SUPER_ADMIN') AND id != $1`, target).Scan(&admins); err != nil {
				fail(w, http.StatusInternalServerError, "gagal memeriksa admin")
				return
			}
			if admins < 1 {
				fail(w, http.StatusBadRequest, "ini admin terakhir — jangan hapus admin terakhir")
				return
			}
		}
		// Pencegahan: jangan biarkan admin menghapus akunnya sendiri.
		if target == uid {
			fail(w, http.StatusBadRequest, "tidak bisa menghapus akun sendiri")
			return
		}
		tag, err := db.Exec(r.Context(), `DELETE FROM "user" WHERE id=$1`, target)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal menghapus akun")
			return
		}
		if tag.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "pengguna tidak ditemukan")
			return
		}
		audit(r.Context(), db, uid, "USER_DELETE", target, nil, nil, ipOf(r))
		ok(w, map[string]any{"ok": true, "id": target, "deleted": true})
	}
}

// AdminSetRole: PATCH /api/admin/users/{id}/role {role}.
//
// Aturan peran ada di SERVER dan tidak boleh hanya di UI: panel adalah
// permukaan yang bisa dipanggil langsung, dan peran adalah hak akses tertinggi
// di platform.
func AdminSetRole(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")
		var in struct {
			Role string `json:"role"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		in.Role = strings.ToUpper(strings.TrimSpace(in.Role))
		if !roles[in.Role] {
			fail(w, http.StatusBadRequest, "peran tidak dikenal (USER, SELLER, ADMIN, SUPER_ADMIN)")
			return
		}
		// Mengubah peran sendiri mengunci panel: yang menurunkan diri kehilangan
		// akses, yang menaikkan diri melewati super admin.
		if id == uid {
			fail(w, http.StatusBadRequest, "tidak bisa mengubah peran sendiri — minta admin lain")
			return
		}
		var targetRole string
		if err := db.QueryRow(r.Context(), `SELECT role FROM "user" WHERE id=$1`, id).Scan(&targetRole); err != nil {
			fail(w, http.StatusNotFound, "pengguna tidak ditemukan")
			return
		}
		if targetRole == in.Role {
			ok(w, map[string]any{"ok": true, "unchanged": true})
			return
		}
		actor := dbRole(r.Context(), db, uid)
		if (in.Role == "SUPER_ADMIN" || targetRole == "SUPER_ADMIN") && actor != "SUPER_ADMIN" {
			fail(w, http.StatusForbidden, "hanya super admin yang boleh memberi atau mencabut peran super admin")
			return
		}
		// Peran ADMIN/SUPER_ADMIN terakhir tidak boleh dicabut: platform tanpa
		// admin tidak bisa dipulihkan dari dalam aplikasi.
		if isAdminRole(targetRole) && !isAdminRole(in.Role) {
			var admins int
			if err := db.QueryRow(r.Context(),
				`SELECT COUNT(*) FROM "user" WHERE role IN ('ADMIN','SUPER_ADMIN')`).Scan(&admins); err != nil {
				fail(w, http.StatusInternalServerError, "gagal memeriksa admin")
				return
			}
			if admins <= 1 {
				fail(w, http.StatusBadRequest, "ini admin terakhir — angkat admin lain dulu")
				return
			}
		}
		if _, err := db.Exec(r.Context(),
			`UPDATE "user" SET role=$1, updated_at=now() WHERE id=$2`, in.Role, id); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mengubah peran")
			return
		}
		audit(r.Context(), db, uid, "USER_ROLE_CHANGE", id, targetRole, in.Role, ipOf(r))
		ok(w, map[string]any{"ok": true, "from": targetRole, "to": in.Role})
	}
}

// AdminSettingsGet: GET /api/admin/settings — platform_setting.
func AdminSettingsGet(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(), `SELECT key, value, updated_at FROM platform_setting ORDER BY key`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		settings := map[string]string{}
		var updated time.Time
		for rows.Next() {
			var k, v string
			var t time.Time
			if err := rows.Scan(&k, &v, &t); err != nil {
				continue
			}
			settings[k] = v
			updated = t
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in AdminSettingsGet: %v", err)
		}
		ok(w, map[string]any{"settings": settings, "updated_at": updated})
	}
}

// AdminSettingsPatch: PATCH /api/admin/settings — upsert pasangan key/value.
func AdminSettingsPatch(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in map[string]any
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		// "settings" dibungkus supaya bentuk lama ({settings:{...}}) tetap jalan.
		if nested, ok := in["settings"].(map[string]any); ok {
			in = nested
		}
		updated := 0
		for k, v := range in {
			if k == "" {
				continue
			}
			var sv string
			switch t := v.(type) {
			case string:
				sv = t
			case json.Number:
				sv = t.String()
			case bool:
				sv = strconv.FormatBool(t)
			default:
				b, err := json.Marshal(t)
				if err != nil {
					continue
				}
				sv = string(b)
			}
			if _, err := db.Exec(r.Context(),
				`INSERT INTO platform_setting (key, value, updated_at) VALUES ($1,$2,now())
				 ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()`, k, sv); err != nil {
				log.Printf("[ERROR] simpan setting %s gagal: %v", k, err)
				continue
			}
			updated++
		}
		if updated == 0 {
			fail(w, http.StatusBadRequest, "tidak ada yang diubah")
			return
		}
		audit(r.Context(), db, uid, "SETTINGS_UPDATE", "", nil, in, ipOf(r))
		ok(w, map[string]any{"ok": true, "updated": updated})
	}
}

// AdminArticles: GET /api/admin/articles?status=.
func AdminArticles(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		if status == "SEMUA" {
			status = ""
		}
		rows, err := db.Query(r.Context(),
			`SELECT a.id, a.title, a.slug, a.content, COALESCE(a.excerpt,''), COALESCE(a.image,''), a.status, a.is_admin_post,
			        a.published_at, a.created_at, a.author_role, COALESCE(u.email,'')
			   FROM article a LEFT JOIN "user" u ON u.id = a.author_id
			  WHERE ($1 = '' OR a.status = $1) ORDER BY a.created_at DESC LIMIT 100`, status)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type a struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Slug    string `json:"slug"`
			Content string `json:"content"`
			Excerpt string `json:"excerpt"`
			Image   string `json:"image,omitempty"`
			Status  string `json:"status"`
			// Halaman /admin/artikel membaca a.published (bukan is_published).
			IsPublished bool       `json:"published"`
			IsAdmin     bool       `json:"is_admin_post"`
			Published   *time.Time `json:"published_at,omitempty"`
			CreatedAt   time.Time  `json:"created_at"`
			AuthorRole  string     `json:"author_role"`
			Author      string     `json:"author"`
		}
		out := []a{}
		for rows.Next() {
			var x a
			if err := rows.Scan(&x.ID, &x.Title, &x.Slug, &x.Content, &x.Excerpt, &x.Image, &x.Status, &x.IsAdmin,
				&x.Published, &x.CreatedAt, &x.AuthorRole, &x.Author); err != nil {
				continue
			}
			x.IsPublished = x.Status == "PUBLISHED"
			x.Content = sanitizeArticleHTML(x.Content)
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in AdminArticles: %v", err)
		}
		ok(w, map[string]any{"articles": out})
	}
}

// AdminArticleWrite: POST /api/admin/articles — buat (atau sunting kalau `id`
// dikirim). Satu jalur untuk dua tindakan karena bentuk datanya identik.
func AdminArticleWrite(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			Content   string `json:"content"`
			Excerpt   string `json:"excerpt"`
			Image     string `json:"image"`
			Published bool   `json:"published"`
			Status    string `json:"status"`
			IsAdmin   bool   `json:"is_admin_post"`
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
		if v := detectLinks(in.Title + "\n" + in.Content + "\n" + in.Excerpt); len(v) > 0 {
			fail(w, http.StatusBadRequest, "link tidak diizinkan: "+strings.Join(v, " | "))
			return
		}
		// Dua bentuk diterima: `published: true` (dipakai UI) dan
		// `status: "published"` (wajar dipakai pemanggil API).
		published := in.Published || strings.EqualFold(in.Status, "PUBLISHED")

		if in.ID != "" {
			var old *time.Time
			if err := db.QueryRow(r.Context(), `SELECT published_at FROM article WHERE id=$1`, in.ID).Scan(&old); err != nil {
				fail(w, http.StatusNotFound, "artikel tidak ditemukan")
				return
			}
			// Tanggal terbit yang sudah ada DIPERTAHANKAN saat menyunting artikel
			// yang sudah terbit — memperbaiki salah ketik tidak boleh melemparnya
			// ke urutan teratas blog.
			var publishedAt any
			if published {
				if old != nil {
					publishedAt = *old
				} else {
					publishedAt = time.Now()
				}
			}
			status := "PENDING"
			if published {
				status = "PUBLISHED"
			}
			if _, err := db.Exec(r.Context(),
				`UPDATE article SET title=$1, content=$2, excerpt=$3, image=$4, status=$5,
				        is_admin_post=$6, published_at=$7, updated_at=now() WHERE id=$8`,
				strings.TrimSpace(in.Title), in.Content, nullStr(in.Excerpt), nullStr(in.Image), status,
				in.IsAdmin, publishedAt, in.ID); err != nil {
				fail(w, http.StatusInternalServerError, "gagal menyimpan artikel")
				return
			}
			audit(r.Context(), db, uid, "ARTICLE_UPDATE", in.ID, nil, map[string]any{"status": status}, ipOf(r))
			ok(w, map[string]any{"ok": true, "id": in.ID})
			return
		}

		id, slug, err := insertArticle(r.Context(), db, uid, "ADMIN", true, strings.TrimSpace(in.Title), in.Content, in.Excerpt, in.Image, published)
		if err != nil {
			log.Printf("[ERROR] buat artikel admin gagal: %v", err)
			fail(w, http.StatusInternalServerError, "gagal menyimpan artikel")
			return
		}
		audit(r.Context(), db, uid, "ARTICLE_CREATE", id, nil, map[string]any{"slug": slug, "published": published}, ipOf(r))
		ok(w, map[string]any{"ok": true, "id": id, "slug": slug})
	}
}

// AdminArticlePatch: PATCH /api/admin/articles/{id} {published} — terbitkan
// atau tarik tanpa menyentuh isinya.
func AdminArticlePatch(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")
		var in struct {
			Published *bool  `json:"published"`
			Status    string `json:"status"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		var published bool
		switch {
		case in.Published != nil:
			published = *in.Published
		case in.Status != "":
			published = strings.EqualFold(in.Status, "PUBLISHED")
		default:
			fail(w, http.StatusBadRequest, "published wajib boolean")
			return
		}

		var old *time.Time
		var oldStatus string
		if err := db.QueryRow(r.Context(),
			`SELECT published_at, status FROM article WHERE id=$1`, id).Scan(&old, &oldStatus); err != nil {
			fail(w, http.StatusNotFound, "artikel tidak ditemukan")
			return
		}
		status := "PENDING"
		var publishedAt any
		if published {
			status = "PUBLISHED"
			if old != nil {
				publishedAt = *old
			} else {
				publishedAt = time.Now()
			}
		}
		if _, err := db.Exec(r.Context(),
			`UPDATE article SET status=$1, published_at=$2, updated_at=now() WHERE id=$3`, status, publishedAt, id); err != nil {
			fail(w, http.StatusInternalServerError, "gagal memperbarui artikel")
			return
		}
		audit(r.Context(), db, uid, "ARTICLE_STATUS", id, oldStatus, status, ipOf(r))
		ok(w, map[string]any{"ok": true, "status": status})
	}
}

// AdminArticleDelete: DELETE /api/admin/articles/{id}. Jejak pelanggaran tautan
// ikut terhapus lewat ON DELETE CASCADE di skema.
func AdminArticleDelete(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		res, err := db.Exec(r.Context(), `DELETE FROM article WHERE id=$1`, r.PathValue("id"))
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal menghapus artikel")
			return
		}
		if res.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "artikel tidak ditemukan")
			return
		}
		audit(r.Context(), db, uid, "ARTICLE_DELETE", r.PathValue("id"), nil, nil, ipOf(r))
		ok(w, map[string]any{"ok": true})
	}
}

// AdminProvidersList sudah ada di api.go; ini pasangan tulisnya.
var _ = provider.HashKey
