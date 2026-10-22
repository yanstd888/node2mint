package handler

// Chat pesanan: terbatas satu order_id, hanya buyer + seller toko pemilik
// order yang bisa baca/kirim. Otorisasi di SELECT (buyer_id / store.user_id).
import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"
)

// OrderChatList: GET /api/market/orders/{id}/chat — semua pesan order ini.
// Hanya buyer atau seller toko yang pemiliki order yang boleh akses.
func OrderChatList(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id, valid := pathUUID(w, r, "id")
		if !valid {
			return
		}
		var buyerID, sellerID string
		err := db.QueryRow(r.Context(),
			`SELECT COALESCE(o.buyer_id::text,''), COALESCE(s.user_id::text,'')
			   FROM market_order o
			   JOIN market_product p ON p.id = o.product_id
			   LEFT JOIN market_store s ON s.id = o.store_id
			  WHERE o.id=$1`, id).
			Scan(&buyerID, &sellerID)
		if err != nil {
			fail(w, http.StatusNotFound, "order tidak ditemukan")
			return
		}
		if uid != buyerID && uid != sellerID {
			fail(w, http.StatusForbidden, "hanya buyer dan seller pesanan ini yang bisa mengakses chat")
			return
		}

		rows, err := db.Query(r.Context(),
			`SELECT c.id, c.sender_id, c.message, c.created_at
			   FROM market_chat c WHERE c.order_id=$1 ORDER BY c.created_at ASC`, id)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()

		type msg struct {
			ID        string    `json:"id"`
			SenderID  string    `json:"sender_id"`
			Message   string    `json:"message"`
			CreatedAt time.Time `json:"created_at"`
			IsMe      bool      `json:"is_me"`
		}
		out := []msg{}
		for rows.Next() {
			var m msg
			if err := rows.Scan(&m.ID, &m.SenderID, &m.Message, &m.CreatedAt); err != nil {
				continue
			}
			m.IsMe = m.SenderID == uid
			out = append(out, m)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in OrderChatList: %v", err)
		}
		ok(w, map[string]any{"order_id": id, "messages": out})
	}
}

// OrderChatSend: POST /api/market/orders/{id}/chat {message} — kirim pesan.
// Hanya buyer atau seller toko yang pemiliki order yang boleh kirim.
func OrderChatSend(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id, valid := pathUUID(w, r, "id")
		if !valid {
			return
		}
		var in struct {
			Message string `json:"message"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		msg := strings.TrimSpace(in.Message)
		if len(msg) == 0 || len(msg) > 2000 {
			fail(w, http.StatusBadRequest, "pesan harus 1–2000 karakter")
			return
		}

		var buyerID, sellerID string
		err := db.QueryRow(r.Context(),
			`SELECT COALESCE(o.buyer_id::text,''), COALESCE(s.user_id::text,'')
			   FROM market_order o
			   JOIN market_product p ON p.id = o.product_id
			   LEFT JOIN market_store s ON s.id = o.store_id
			  WHERE o.id=$1`, id).
			Scan(&buyerID, &sellerID)
		if err != nil {
			fail(w, http.StatusNotFound, "order tidak ditemukan")
			return
		}
		if uid != buyerID && uid != sellerID {
			fail(w, http.StatusForbidden, "hanya buyer dan seller pesanan ini yang bisa mengirim")
			return
		}

		var chatID string
		if err := db.QueryRow(r.Context(),
			`INSERT INTO market_chat (order_id, sender_id, message) VALUES ($1,$2,$3)
			   RETURNING id`, id, uid, msg).
			Scan(&chatID); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan pesan")
			return
		}
		audit(r.Context(), db, uid, "ORDER_CHAT_SEND", id, nil, msg, ipOf(r))
		// Notifikasi ke pihak lawan (async, best-effort).
		var recipientID string
		if uid == buyerID {
			recipientID = sellerID
		} else {
			recipientID = buyerID
		}
		// Ambil nama pengirim untuk notifikasi (best-effort, kosongkan jika gagal).
		var senderName string
		_ = db.QueryRow(r.Context(), `SELECT COALESCE(name,'') FROM "user" WHERE id=$1`, uid).Scan(&senderName)
		// Ambil judul produk untuk notifikasi.
		var title string
		_ = db.QueryRow(r.Context(), `SELECT COALESCE(p.title,'') FROM market_order o JOIN market_product p ON p.id=o.product_id WHERE o.id=$1`, id).Scan(&title)
		go genOrderChatNotification(context.Background(), db, id, recipientID, title, senderName)
		ok(w, map[string]any{"ok": true, "id": chatID, "sender_id": uid, "message": msg})
	}
}

// ReconcileOverdueOrders: memindai order non-instan PAID/PENDING yang
// melewati process_deadline 24h, lalu batalkan + refund + kembalikan stok
// secara atomik. Scheduler menjalankannya tiap menit; endpoint order juga
// memanggilnya agar hasil segera konsisten ketika halaman dibuka.
//
// Pemangkasan tabel (audit/idempotency/notifikasi) TIDAK di sini: ini berjalan
// juga di jalur request, dan DELETE massal di jalur request membuat satu
// pembukaan halaman menunggu lock tabel. Prune dijalankan scheduler.
//
// Daftar id dibaca habis dulu ke slice, baru diproses — cancelOrder membuka
// transaksi yang butuh koneksi pool kedua, dan cursor yang masih terbuka
// menahan koneksi pertama.
func ReconcileOverdueOrders(ctx context.Context, db DB) int {
	ids, err := scanIDs(ctx, db,
		`SELECT o.id FROM market_order o
		   JOIN market_product p ON p.id=o.product_id
		  WHERE o.status IN ('PAID','PENDING')
		    AND NOT p.is_instant
		    AND o.process_deadline IS NOT NULL
		    AND o.process_deadline < now()
		  ORDER BY o.process_deadline ASC LIMIT 50`)
	if err != nil {
		log.Printf("[WARN] reconcile overdue query failed: %v", err)
		return 0
	}

	processed := 0
	for _, id := range ids {
		// admin=true agar tidak perlu mengecek owner; cancelOrder sudah
		// memakai FOR UPDATE dan transaksi atomik (refund + stok + instan item).
		if code, _ := cancelOrder(ctx, db, id, "", true, "Auto-cancel: seller belum merespon/delivery dalam 24h", "", ""); code == 0 {
			processed++
		}
	}
	if processed > 0 {
		log.Printf("[INFO] ReconcileOverdueOrders: auto-cancelled %d orders", processed)
	}
	return processed
}

const pruneAuditLogSQL = `DELETE FROM audit_log WHERE created_at < now() - interval '14 days'`
const pruneIdempotencySQL = `DELETE FROM idempotency_request WHERE created_at < now() - interval '30 days'`
const pruneNotificationsSQL = `DELETE FROM notifications WHERE created_at < now() - interval '14 days'`
const pruneKYCVerificationSQL = `DELETE FROM kyc_verification WHERE status IN ('approved','rejected') AND reviewed_at < now() - interval '30 days'`

// Retention prune: dijalankan dari scheduler order, BUKAN dari jalur
// request — ReconcileOverdueOrders juga dipanggil saat halaman order dibuka,
// dan DELETE massal di jalur request membuat satu kali buka halaman menunggu
// lock tabel. Prune dijalankan scheduler.
// ponytail: satu kali per tick (1 menit); beri throttle per jam kalau tabel
// notifikasi sudah besar — DELETE-nya seq-scan karena tidak ada indeks
// created_at tunggal.
func pruneAuditLog(ctx context.Context, db DB) {
	if _, err := db.Exec(ctx, pruneAuditLogSQL); err != nil {
		log.Printf("[WARN] audit log prune failed: %v", err)
	}
	if _, err := db.Exec(ctx, pruneIdempotencySQL); err != nil {
		log.Printf("[WARN] idempotency prune failed: %v", err)
	}
	if _, err := db.Exec(ctx, pruneNotificationsSQL); err != nil {
		log.Printf("[WARN] notification prune failed: %v", err)
	}
	if _, err := db.Exec(ctx, pruneKYCVerificationSQL); err != nil {
		log.Printf("[WARN] KYC document prune failed: %v", err)
	}
}

// StartOrderReconciler memastikan pembatalan + pelepasan escrow tetap berjalan
// walau tidak ada buyer/admin yang sedang membuka halaman pesanan. Retention
// prune ikut di sini — satu-satunya tempat yang bukan jalur request.
func StartOrderReconciler(db DB, every time.Duration) {
	go func() {
		ReconcileOverdueOrders(context.Background(), db)
		ReleaseDueEscrows(context.Background(), db)
		pruneAuditLog(context.Background(), db)
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for range ticker.C {
			ReconcileOverdueOrders(context.Background(), db)
			ReleaseDueEscrows(context.Background(), db)
			pruneAuditLog(context.Background(), db)
		}
	}()
}

// AdminReconcile: POST /api/admin/orders/reconcile — paksa jalankan auto-cancel
// order yang sudah melewati deadline 24h. Admin-only.
func AdminReconcile(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		if !isAdminRole(dbRole(r.Context(), db, uid)) {
			fail(w, http.StatusForbidden, "admin akses diperlukan")
			return
		}
		n := ReconcileOverdueOrders(r.Context(), db)
		audit(r.Context(), db, uid, "ORDER_RECONCILE", "", nil, map[string]any{"cancelled": n}, ipOf(r))
		ok(w, map[string]any{"ok": true, "cancelled": n})
	}
}
