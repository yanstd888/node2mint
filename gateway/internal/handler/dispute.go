package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// Sengketa pasar: buyer mengangkat, admin menyelesaikan.
//
// Kebijakan (ditulis di schema.sql + di sini):
//   - Buyer hanya bisa mengangkat sengketa pada order PAID/PROCESSING, atau
//     COMPLETED yang belum lanjut 7 hari (dicek berdasarkan completed_at).
//   - Satu order = satu sengketa (uq_dispute_order).
//   - Admin buyer_wins → kembalikan dana ke pembeli DARI SALDO TOKO seller:
//     hanya bila cukup; bila kurang → manual_review (tidak potong sebagian).
//   - Tidak ada refund parsial: buyer_wins membatalkan semua, atau tidak sama sekali.
//   - Seller mendapat tenggat respons informasional 24 jam sejak sengketa dibuat;
//     lewatnya tenggat tidak mengubah escrow atau status secara otomatis.

// disputeEligible memeriksa apakah buyer boleh membuka sengketa pada order ini.
// Dipisahkan dari handler supaya bisa diuji tanpa DB connection.
func disputeEligibility(status string, completedAt *time.Time, now time.Time) bool {
	switch status {
	case "PAID", "PROCESSING":
		return true
	case "COMPLETED":
		if completedAt == nil {
			return false
		}
		// Batas 7 hari setelah penyelesaian — setelah itu pembeli ditutup.
		return now.Sub(*completedAt) <= 7*24*time.Hour
	default:
		return false
	}
}

// resolveDecision: keputusan admin. buyerWins=true & storeBal >= orderSubtotal →
// kembalikan penuh ke buyer dari saldo toko. Jika saldo toko < subtotal →
// manual_review. Jika !buyerWins → RESOLVED (tutup sengketa, escrow dilepas ke
// seller lewat releaseEscrow sehingga order masuk COMPLETED yang sah).
// Tidak ada jalur "refund sebagian".
func resolveDecision(buyerWins bool, storeBalance, orderSubtotal float64) string {
	if !buyerWins {
		return "RESOLVED" // seller wins — dispute closed, escrow lanjut normal
	}
	if storeBalance >= orderSubtotal {
		return "BUYER_WINS"
	}
	return "MANUAL_REVIEW"
}

// nextOrderStatus: transisi order setelah sengketa diputuskan, atau "" kalau
// order harus dibiarkan apa adanya.
//   - BUYER_WINS → CANCELLED (refund penuh, stok kembali, escrow tidak pernah cair).
//   - RESOLVED & order masih DISPUTED → PAID, supaya releaseEscrow punya status
//     sah untuk dilepas; tanpa ini seller menang tapi tidak pernah dibayar.
//   - MANUAL_REVIEW → "" : order tetap DISPUTED, jadi escrow tetap tertahan.
func nextOrderStatus(dispute, order string) string {
	switch {
	case dispute == "BUYER_WINS":
		return "CANCELLED"
	case dispute == "RESOLVED" && order == "DISPUTED":
		return "PAID"
	default:
		return ""
	}
}

// A completed order is changed to DISPUTED while it is reviewed, so the
// status alone cannot tell whether escrow was already paid. completed_at is
// the durable marker that prevents a second seller payout or read-code
// restock after a post-completion dispute.
func disputeOrderHadSettledEscrow(orderStatus string, completedAt *time.Time) bool {
	return orderStatus == "COMPLETED" || completedAt != nil
}

// CreateDispute: POST /api/market/orders/{id}/dispute {reason}.
// Buyer-only (dijaga middleware + pengecekan kepemilikan di sini).
func CreateDispute(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")

		var in struct {
			Reason      string `json:"reason"`
			EvidenceURL string `json:"evidence_url,omitempty"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		if len(in.Reason) < 10 || len(in.Reason) > 1000 {
			fail(w, http.StatusBadRequest, "reason minimal 10 karakter")
			return
		}
		if len(in.EvidenceURL) > 500 {
			fail(w, http.StatusBadRequest, "evidence_url terlalu panjang")
			return
		}

		var buyer, storeID, status string
		var subtotal float64
		var completedAt *time.Time
		err := db.QueryRow(r.Context(),
			`SELECT COALESCE(o.buyer_id::text,''), COALESCE(o.store_id::text,''), o.status, o.subtotal, o.completed_at
			   FROM market_order o WHERE o.id=$1`,
			id).Scan(&buyer, &storeID, &status, &subtotal, &completedAt)
		if err != nil {
			fail(w, http.StatusNotFound, "order tidak ditemukan")
			return
		}
		if buyer != uid {
			fail(w, http.StatusForbidden, "bukan pesanan Anda")
			return
		}
		if !disputeEligibility(status, completedAt, time.Now()) {
			fail(w, http.StatusConflict, "order tidak dapat dibuka sengketa")
			return
		}

		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		defer tx.Rollback(r.Context())

		var did string
		if err := tx.QueryRow(r.Context(),
			`SELECT o.status, o.completed_at FROM market_order o WHERE o.id=$1 AND o.buyer_id=$2 FOR UPDATE`, id, uid).
			Scan(&status, &completedAt); err != nil {
			fail(w, http.StatusConflict, "order tidak dapat dibuka sengketa")
			return
		}
		if !disputeEligibility(status, completedAt, time.Now()) {
			fail(w, http.StatusConflict, "order tidak dapat dibuka sengketa")
			return
		}
		err = tx.QueryRow(r.Context(),
			`INSERT INTO market_dispute (order_id, buyer_id, store_id, reason, evidence_url, response_deadline) VALUES ($1,$2,$3,$4,$5,now()+interval '24 hours')
			 ON CONFLICT (order_id) DO NOTHING RETURNING id`, id, uid, nullStr(storeID), in.Reason, nullStr(in.EvidenceURL)).Scan(&did)
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, http.StatusConflict, "sengketa sudah ada")
			return
		}
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal membuat sengketa")
			return
		}
		updated, err := tx.Exec(r.Context(),
			`UPDATE market_order SET status='DISPUTED', updated_at=now() WHERE id=$1 AND status=$2`, id, status)
		if err != nil || updated.RowsAffected() == 0 {
			fail(w, http.StatusConflict, "status order berubah")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan sengketa")
			return
		}
		audit(r.Context(), db, uid, "DISPUTE_OPEN", did, status, "DISPUTED", ipOf(r))
		// Notify seller (store owner) + admin about new dispute.
		evidence := ""
		if in.EvidenceURL != "" {
			evidence += in.EvidenceURL + ". "
		}
		go insertNotificationToAdmins(r.Context(), db, "Sengketa baru pada pesanan Anda",
			"Pembeli membuka sengketa. Bukti: "+evidence)
		if storeID != "" {
			go genSystemNotification(context.Background(), db, storeID, "Sengketa baru pada toko Anda",
				"Pembeli membuka sengketa pada pesanan Anda. Bukti: "+evidence)
			go queueDisputeSellerNotification(context.Background(), db, did, id, storeID, in.Reason, subtotal)
		}
		ok(w, map[string]any{"ok": true, "id": did, "status": "DISPUTED"})
	}
}

// ResolveDispute: POST /api/admin/disputes/{id}/resolve {buyer_wins, resolution}.
// buyer_wins → kembalikan dana dari saldo toko seller (cek cukup/tidak).
// Saldo toko < subtotal → manual_review (tidak memotong). !buyerWins → RESOLVED.
// ponytail: read-before-write race di store_balance antara SELECT dan UPDATE;
//
//	transaksi UPDATE ... WHERE wallet_balance >= subtotal menjamin atomisitas
//	pada potongan. Race window kecil pada nilai bacaan keputusan (decision) —
//	cukup karena manual_review justru fallback aman. Naikkan ke FOR UPDATE bila
//	concurrency tinggi.
func ResolveDispute(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")

		var in struct {
			BuyerWins  bool   `json:"buyer_wins"`
			Resolution string `json:"resolution,omitempty"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		if len(in.Resolution) > 1000 {
			fail(w, http.StatusBadRequest, "resolution maksimal 1000 karakter")
			return
		}

		// Satu transaksi mengunci dispute DAN order DAN saldo toko: keputusan
		// refund tidak boleh dihitung dari saldo yang dapat berubah sebelum
		// debit, dan order seller-wins harus lanjut ke penyelesaian yang sah
		// tanpa jendela di mana order sudah terbuka tapi dana belum pindah.
		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		defer tx.Rollback(r.Context())

		var orderID, buyerID, storeID, orderStatus string
		var subtotal float64
		var completedAt *time.Time
		err = tx.QueryRow(r.Context(),
			`SELECT d.order_id::text, d.buyer_id::text, COALESCE(d.store_id::text,''), o.subtotal, o.status, o.completed_at
			   FROM market_dispute d JOIN market_order o ON o.id=d.order_id WHERE d.id=$1 FOR UPDATE OF d, o`, id).
			Scan(&orderID, &buyerID, &storeID, &subtotal, &orderStatus, &completedAt)
		if err != nil {
			fail(w, http.StatusNotFound, "sengketa tidak ditemukan")
			return
		}
		var storeBalance float64
		// Sengketa tanpa toko tidak punya saldo yang bisa didebit maupun
		// dikredit: jangan lanjut dengan saldo 0 palsu yang membuat buyer_wins
		// tampak berhasil.
		if storeID == "" {
			fail(w, http.StatusConflict, "Sengketa tanpa toko — tidak ada saldo seller untuk refund; perbaiki store_id order dulu")
			return
		}
		if err := tx.QueryRow(r.Context(), `SELECT wallet_balance FROM market_store WHERE id=$1 FOR UPDATE`, storeID).Scan(&storeBalance); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mengunci saldo toko")
			return
		}
		decision := resolveDecision(in.BuyerWins, storeBalance, subtotal)

		var newStatus string
		switch decision {
		case "BUYER_WINS":
			newStatus = "BUYER_WINS"
		case "MANUAL_REVIEW":
			newStatus = "MANUAL_REVIEW"
		default:
			newStatus = "RESOLVED"
		}
		updated, err := tx.Exec(r.Context(),
			`UPDATE market_dispute SET status=$1, resolution=$2, resolved_by=$3, resolved_at=now(), updated_at=now() WHERE id=$4 AND status IN ('OPEN','MANUAL_REVIEW')`,
			newStatus, in.Resolution, uid, id)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memperbarui sengketa")
			return
		}
		if updated.RowsAffected() == 0 {
			fail(w, http.StatusConflict, "sengketa sudah diproses")
			return
		}

		// Jika buyer_wins → refund penuh ke buyer DARI SALDO TOKO seller.
		if newStatus == "BUYER_WINS" {
			refunded, err := doBuyerWinsRefund(r.Context(), tx, storeID, buyerID, subtotal, orderID)
			if err != nil {
				fail(w, http.StatusInternalServerError, "gagal memproses refund: "+err.Error())
				return
			}
			if !refunded {
				// Race: saldo toko habis sejak pembacaan keputusan.
				if _, err := tx.Exec(r.Context(),
					`UPDATE market_dispute SET status='MANUAL_REVIEW' WHERE id=$1`, id); err != nil {
					fail(w, http.StatusInternalServerError, "gagal rollback sengketa ke manual_review")
					return
				}
				newStatus = "MANUAL_REVIEW"
			}
		}

		// Transisi order ditentukan satu fungsi (nextOrderStatus) supaya tidak
		// ada dua tempat yang menghitung status akhir. Karena order COMPLETED
		// menjadi DISPUTED saat sengketa dibuka, completed_at menjadi penentu
		// apakah payout/produk sudah pernah diselesaikan sebelumnya.
		wasCompleted := disputeOrderHadSettledEscrow(orderStatus, completedAt)
		if next := nextOrderStatus(newStatus, orderStatus); next == "CANCELLED" {
			if _, err := tx.Exec(r.Context(),
				`UPDATE market_order SET status='CANCELLED', refunded_at=now(), updated_at=now() WHERE id=$1`,
				orderID); err != nil {
				fail(w, http.StatusInternalServerError, "gagal memperbarui order")
				return
			}
			// Order yang sudah COMPLETED telah membayar seller dan mengirim
			// barang; refund sengketa tidak boleh memasukkan kembali stok atau
			// melepas kode digital yang sudah dibaca buyer.
			if !wasCompleted {
				if err := restockOrder(r.Context(), tx, orderID); err != nil {
					fail(w, http.StatusInternalServerError, "gagal mengembalikan stok order")
					return
				}
			}
		} else if next == "PAID" {
			// Seller menang setelah payout sebelumnya sudah selesai: tutup
			// sengketa dan pulihkan COMPLETED tanpa membayar dua kali.
			if wasCompleted {
				if _, err := tx.Exec(r.Context(),
					`UPDATE market_order SET status='COMPLETED', updated_at=now() WHERE id=$1 AND status='DISPUTED'`, orderID); err != nil {
					fail(w, http.StatusInternalServerError, "gagal memulihkan order selesai")
					return
				}
			} else {
				// Seller menang: escrow boleh lanjut. Hapus status DISPUTED dulu
				// supaya releaseEscrow punya status sah, lalu bayar seller DALAM
				// transaksi ini juga — kalau tidak, order sempat menggantung di
				// PAID dan auto-cancel 24 jam bisa membatalkannya.
				if _, err := tx.Exec(r.Context(),
					`UPDATE market_order SET status='PAID', updated_at=now() WHERE id=$1 AND status='DISPUTED'`, orderID); err != nil {
					fail(w, http.StatusInternalServerError, "gagal melanjutkan order")
					return
				}
				if _, reason, fine := releaseEscrowTx(r.Context(), tx, orderID); !fine {
					fail(w, http.StatusConflict, reason)
					return
				}
			}
		}

		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan penyelesaian")
			return
		}
		audit(r.Context(), db, uid, "DISPUTE_RESOLVE", id, decision, newStatus, ipOf(r))
		// Notify buyer + seller on resolution.
		go genDisputeResolvedNotification(r.Context(), db, buyerID, storeID, orderID, newStatus)
		ok(w, map[string]any{
			"ok":         true,
			"dispute_id": id,
			"decision":   newStatus,
		})
	}
}

// doBuyerWinsRefund mengpotong saldo toko & mengembalikan ke buyer.
// Kembali refunded=false bila saldo toko tak cukup (race condition).
func doBuyerWinsRefund(ctx context.Context, tx pgx.Tx, storeID, buyerID string, subtotal float64, invoiceID string) (bool, error) {
	if storeID == "" || buyerID == "" || subtotal <= 0 {
		return false, fmt.Errorf("store, buyer, atau subtotal tidak valid")
	}
	// Ponytail: UPDATE dengan WHERE wallet_balance >= subtotal — atomis, tidak
	// memotong sebagian. RowsAffected()==0 berarti saldo tidak cukup.
	res, err := tx.Exec(ctx,
		`UPDATE market_store SET wallet_balance = wallet_balance - $1, updated_at=now() WHERE id=$2 AND wallet_balance >= $1`,
		subtotal, storeID)
	if err != nil {
		return false, err
	}
	if ra := res.RowsAffected(); ra == 0 {
		return false, nil // saldo tidak cukup → caller akan set MANUAL_REVIEW
	}
	// Catat refund ke buyer (credit_transaction) & tambah saldo buyer.
	if buyerID != "" {
		if _, err := tx.Exec(ctx,
			`INSERT INTO credit_transaction (user_id, amount, type, status, description, invoice_id)
			 VALUES ($1,$2,'REFUND','COMPLETED',$3,$4)`,
			buyerID, subtotal, "Refund sengketa — buyer wins", invoiceID); err != nil {
			return false, fmt.Errorf("gagal mencatat refund: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE "user" SET wallet_balance = wallet_balance + $1 WHERE id=$2`,
			subtotal, buyerID); err != nil {
			return false, fmt.Errorf("gagal menambah saldo buyer: %w", err)
		}
	}
	return true, nil
}

// GetDispute: GET /api/market/orders/{id}/dispute — buyer lihat status sengketa.
// Buyer-only (harus pemilik order).
func GetDispute(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathUUID(w, r, "id")
		if !valid {
			return
		}
		uid := uidFromCtx(r.Context())
		type d struct {
			ID             string `json:"id"`
			Status         string `json:"status"`
			Reason         string `json:"reason"`
			EvidenceURL    string `json:"evidence_url,omitempty"`
			SellerResponse string `json:"seller_response,omitempty"`
			Resolution     string `json:"resolution,omitempty"`
			CreatedAt      string `json:"created_at"`
		}
		var x d
		err := db.QueryRow(r.Context(),
			`SELECT d.id, d.status, COALESCE(d.reason,''), COALESCE(d.evidence_url,''), COALESCE(d.seller_response,''), COALESCE(d.resolution,''), d.created_at
			   FROM market_dispute d
			   JOIN market_order o ON o.id = d.order_id
			  WHERE d.order_id = $1 AND o.buyer_id = $2`,
			id, uid).Scan(&x.ID, &x.Status, &x.Reason, &x.EvidenceURL, &x.SellerResponse, &x.Resolution, &x.CreatedAt)
		if err != nil {
			fail(w, http.StatusNotFound, "sengketa tidak ditemukan")
			return
		}
		ok(w, map[string]any{"dispute": x})
	}
}

// SellerRespond: POST /api/market/orders/{id}/dispute/respond {response}.
// Seller-only: harus pemilik toko pada order ini. Hanya boleh saat status OPEN.
func SellerRespond(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathUUID(w, r, "id")
		if !valid {
			return
		}
		uid := uidFromCtx(r.Context())
		var in struct {
			Response string `json:"response"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		if len(in.Response) < 10 || len(in.Response) > 2000 {
			fail(w, http.StatusBadRequest, "response minimal 10 karakter, maksimal 2000")
			return
		}
		res, err := db.Exec(r.Context(),
			`UPDATE market_dispute d
			 SET seller_response = $1, seller_response_at = now(), updated_at = now()
			 WHERE d.order_id = $2
			   AND d.status = 'OPEN'
			   AND EXISTS (SELECT 1 FROM market_order o WHERE o.id = d.order_id AND o.store_id IN (
			           SELECT id FROM market_store WHERE user_id = $3))`,
			in.Response, id, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan respon")
			return
		}
		if res.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "sengketa tidak ditemukan, bukan milik toko Anda, atau sudah selesai")
			return
		}
		audit(r.Context(), db, uid, "DISPUTE_SELLER_RESPONSE", id, nil, in.Response, ipOf(r))
		ok(w, map[string]any{"ok": true})
	}
}

// StoreDisputes: GET /api/store/disputes — sengketa pada order di toko sendiri.
func StoreDisputes(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		s, found := loadStore(r.Context(), db, uid)
		if !found {
			fail(w, http.StatusNotFound, "toko belum terdaftar")
			return
		}
		limit, offset, page, per := pageOf(r)
		rows, err := db.Query(r.Context(),
			`SELECT d.id, d.order_id::text, COALESCE(d.buyer_id::text,''), COALESCE(d.store_id::text,''),
			        d.reason, d.status, COALESCE(d.resolution,''), COALESCE(d.evidence_url,''),
			        COALESCE(d.seller_response,''), d.seller_response_at, d.created_at, o.subtotal,
			        COALESCE(u.email,''), COALESCE(p.title,'')
			   FROM market_dispute d
			   JOIN market_order o ON o.id = d.order_id
			   LEFT JOIN "user" u ON u.id = d.buyer_id
			   LEFT JOIN market_product p ON p.id = o.product_id
			  WHERE d.store_id = $1
			  ORDER BY d.created_at DESC LIMIT $2 OFFSET $3`,
			s.ID, limit, offset)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type d struct {
			ID               string    `json:"id"`
			OrderID          string    `json:"order_id"`
			BuyerID          string    `json:"buyer_id"`
			Reason           string    `json:"reason"`
			Status           string    `json:"status"`
			Resolution       string    `json:"resolution,omitempty"`
			EvidenceURL      string    `json:"evidence_url,omitempty"`
			SellerResponse   string    `json:"seller_response,omitempty"`
			SellerResponseAt string    `json:"seller_response_at,omitempty"`
			CreatedAt        time.Time `json:"created_at"`
			Subtotal         float64   `json:"subtotal"`
			BuyerEmail       string    `json:"buyer_email"`
			ProductTitle     string    `json:"product_title"`
		}
		out := []d{}
		for rows.Next() {
			var x d
			var srAt *time.Time
			if err := rows.Scan(&x.ID, &x.OrderID, &x.BuyerID, &x.Reason, &x.Status, &x.Resolution, &x.EvidenceURL, &x.SellerResponse, &srAt, &x.CreatedAt, &x.Subtotal, &x.BuyerEmail, &x.ProductTitle); err != nil {
				continue
			}
			if srAt != nil {
				x.SellerResponseAt = srAt.Format(time.RFC3339)
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in StoreDisputes: %v", err)
		}
		more := len(out) > per
		if more {
			out = out[:len(out)-1]
		}
		ok(w, map[string]any{"disputes": out, "page": page, "has_more": more})
	}
}

// DisputeList (admin): GET /api/admin/disputes?status=OPEN — daftar sengketa.
func DisputeList(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusFilter := r.URL.Query().Get("status")
		limit, offset, page, per := pageOf(r)

		wheres := "WHERE 1=1"
		args := []any{}
		if statusFilter != "" {
			args = append(args, statusFilter)
			wheres = fmt.Sprintf("WHERE d.status = $%d", len(args))
		}
		args = append(args, limit, offset)
		nArgs := len(args)

		rows, err := db.Query(r.Context(),
			fmt.Sprintf(`SELECT d.id, d.order_id::text, d.buyer_id::text, COALESCE(d.store_id::text,''),
					d.reason, d.status, COALESCE(d.resolution,''), COALESCE(d.evidence_url,''), COALESCE(d.seller_response,''), d.seller_response_at, d.created_at, o.subtotal, COALESCE(u.email,''), COALESCE(s.name,'')
			  FROM market_dispute d
			  JOIN market_order o ON o.id = d.order_id
			  LEFT JOIN "user" u ON u.id = d.buyer_id
			  LEFT JOIN market_store s ON s.id = d.store_id
			  %s ORDER BY d.created_at DESC LIMIT $%d OFFSET $%d`, wheres, nArgs-1, nArgs),
			args...)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()

		type d struct {
			ID               string    `json:"id"`
			OrderID          string    `json:"order_id"`
			BuyerID          string    `json:"buyer_id"`
			StoreID          string    `json:"store_id,omitempty"`
			Reason           string    `json:"reason"`
			Status           string    `json:"status"`
			Resolution       string    `json:"resolution,omitempty"`
			EvidenceURL      string    `json:"evidence_url,omitempty"`
			SellerResponse   string    `json:"seller_response,omitempty"`
			SellerResponseAt string    `json:"seller_response_at,omitempty"`
			CreatedAt        time.Time `json:"created_at"`
			Subtotal         float64   `json:"subtotal"`
			BuyerEmail       string    `json:"buyer_email"`
			StoreName        string    `json:"store_name"`
		}
		out := []d{}
		for rows.Next() {
			var x d
			var srAt *time.Time
			if err := rows.Scan(&x.ID, &x.OrderID, &x.BuyerID, &x.StoreID, &x.Reason, &x.Status, &x.Resolution, &x.EvidenceURL, &x.SellerResponse, &srAt, &x.CreatedAt, &x.Subtotal, &x.BuyerEmail, &x.StoreName); err != nil {
				continue
			}
			if srAt != nil {
				x.SellerResponseAt = srAt.Format(time.RFC3339)
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in DisputeList: %v", err)
		}
		more := len(out) > per
		if more {
			out = out[:len(out)-1]
		}
		ok(w, map[string]any{"disputes": out, "page": page, "has_more": more})
	}
}
