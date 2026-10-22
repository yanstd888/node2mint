package handler

// Notifikasi dashboard bell. Satu tabel, selalu discoping oleh user_id di
// query — tidak ada jalur SELECT/UPDATE yang bisa melewati user_id.
//
// Tipe: PURCHASE (beli produk), SALE (produk terjual), ORDER_CHAT (pesan order),
// SYSTEM (update/pesan sistem), AFFILIATE_COMMISSION (komisi afiliasi masuk).
// Admin dapat membuat notifikasi custom.
import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- Tipe & konstanta ---

type notifType string

const (
	NotifPurchase  notifType = "PURCHASE"
	NotifSale      notifType = "SALE"
	NotifOrderChat notifType = "ORDER_CHAT"
	NotifSystem    notifType = "SYSTEM"
	// Kredit komisi afiliasi ke dompet affiliator saat escrow order cair.
	// Bukan SYSTEM: event uang punya tipe sendiri, sama seperti PURCHASE/SALE.
	NotifAffiliateCommission notifType = "AFFILIATE_COMMISSION"
)

// Notif adalah representasi JSON dari satu baris notifications.
type Notif struct {
	ID        string    `json:"id"`
	Type      notifType `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
	// OrderID / metadata opsional, hanya diisi untuk tipe terkait order.
	OrderID *string `json:"order_id,omitempty"`
}

// notifIn adalah payload untuk membuat notifikasi (admin atau internal).
type notifIn struct {
	UserID  string  `json:"user_id"` // kosong = all users (admin only)
	Title   string  `json:"title"`
	Message string  `json:"message"`
	OrderID *string `json:"order_id,omitempty"`
	// Type hanya boleh di-set admin; internal event selalu pakai tipe spesifik.
	Type notifType `json:"type"`
}

// --- Query DB: SEMUASELALU discoping user_id ---
// pageOf(limit,offset) membaca ?page=&per_page= (default 1 & 20, max 100).
// LIMIT = per+1 agar klien tahu ada halaman berikutnya tanpa COUNT(*).

func listNotifications(ctx context.Context, db DB, userID string, limit, offset int) ([]Notif, bool, error) {
	rows, err := db.Query(ctx,
		`SELECT id, type, title, message, is_read, created_at, order_id
		   FROM notifications
		  WHERE user_id=$1
		   ORDER BY created_at DESC
		   LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	out := []Notif{}
	for rows.Next() {
		var n Notif
		var orderID sql.NullString
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Message, &n.IsRead, &n.CreatedAt, &orderID); err != nil {
			return nil, false, err
		}
		if orderID.Valid {
			n.OrderID = &orderID.String
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	// Trim the extra "probe" row so the client gets exactly `per` items max.
	hasMore := len(out) > limit-1
	if hasMore {
		out = out[:limit-1]
	}
	return out, hasMore, nil
}

func markNotificationRead(ctx context.Context, db DB, userID, notifID string) error {
	// UPDATE selalu ada WHERE user_id=$1 — tidak ada cara untuk menimpa baris user lain.
	tag, err := db.Exec(ctx,
		`UPDATE notifications SET is_read=true WHERE id=$1 AND user_id=$2`, notifID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

const markAllNotificationsReadSQL = `UPDATE notifications SET is_read=true WHERE user_id=$1 AND is_read=false`

// markAllNotificationsRead menandai SEMUA notifikasi user sekaligus (single UPDATE).
// Dipanggil saat lonceng dibuka — "automatic bell read".
func markAllNotificationsRead(ctx context.Context, db DB, userID string) error {
	_, err := db.Exec(ctx, markAllNotificationsReadSQL, userID)
	return err
}

// execer: subset Exec yang dimiliki *pgxpool.Pool DAN pgx.Tx. Dipakai supaya
// notifikasi bisa ditulis di dalam transaksi yang SAMA dengan pergerakan uang
// (kredit komisi) — bukan lewat koneksi kedua yang bisa gagal terpisah dan
// meninggalkan baris notif tanpa kreditnya.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// insertNotification membuat satu baris. user_id wajib di-isi.
func insertNotification(ctx context.Context, db execer, userID string, typ notifType, title, message string, orderID *string, dispatchID string) bool {
	_, err := db.Exec(ctx,
		`INSERT INTO notifications (user_id, type, title, message, order_id, dispatch_id)
		   VALUES ($1,$2,$3,$4,$5,$6)`, userID, string(typ), title, message, orderID, dispatchID)
	return err == nil
}

// insertNotificationToAll makes one row per user.
// ponytail: scan semua user_id; untuk skala ribuan+ ganti batch INSERT ... SELECT FROM "user".
func insertNotificationToAll(ctx context.Context, db DB, typ notifType, title, message string, orderID *string) {
	dispatchID := randHex(16)
	rows, err := db.Query(ctx, `SELECT id FROM "user" WHERE role IN ('USER','SELLER')`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		insertNotification(ctx, db, id, typ, title, message, orderID, dispatchID)
	}
	if typ == NotifSystem {
		go sendTelegramAdmin(context.Background(), title+"\n"+message)
	}
}

// --- Handler publik (user) ---

// ListNotifications: GET /api/notifications?per_page=&page=
// Pagination via pageOf (default 20, max 100). Returns has_more so the
// client can fetch the next page without a COUNT(*).
func ListNotifications(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		if uid == "" {
			browserError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		limit, offset, _, _ := pageOf(r)
		out, hasMore, err := listNotifications(r.Context(), db, uid, limit, offset)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memuat notifikasi")
			return
		}
		ok(w, map[string]any{"notifications": out, "has_more": hasMore})
	}
}

// MarkAllNotificationsRead: POST /api/notifications/read-all
// Automatic bell read: marks every unread notification for the user as read
// in a single UPDATE (WHERE user_id=$1, so it can never touch another user's rows).
func MarkAllNotificationsRead(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		if uid == "" {
			browserError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := markAllNotificationsRead(r.Context(), db, uid); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menandai semua dibaca")
			return
		}
		ok(w, map[string]any{"ok": true})
	}
}

// MarkNotificationRead: POST /api/notifications/{id}/read
func MarkNotificationRead(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		if uid == "" {
			browserError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		notifID := r.PathValue("id")
		if notifID == "" {
			fail(w, http.StatusBadRequest, "id notifikasi wajib")
			return
		}
		if err := markNotificationRead(r.Context(), db, uid, notifID); err != nil {
			fail(w, http.StatusNotFound, "notifikasi tidak ditemukan")
			return
		}
		ok(w, map[string]any{"ok": true})
	}
}

// UnreadNotificationCount: GET /api/notifications/unread
// Dipakai AppShell untuk badge count. Ringan — hanya COUNT.
func UnreadNotificationCount(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		if uid == "" {
			browserError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var count int
		err := db.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND is_read=false`, uid).Scan(&count)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal menghitung")
			return
		}
		ok(w, map[string]any{"unread": count})
	}
}

// --- Handler admin ---

// AdminListNotifications: GET /api/admin/notifications?user_id=&type=&unread=
func AdminListNotifications(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// Admin melihat notifikasi user mana saja — scope via query param user_id.
		// Tanpa user_id: semua notifikasi (untuk audit). Admin sudah terauth.
		uidParam := r.URL.Query().Get("user_id")
		typeParam := r.URL.Query().Get("type")
		unreadParam := r.URL.Query().Get("unread")
		limit, offset, _, _ := pageOf(r)

		query := `SELECT COALESCE(dispatch_id,id::text), MIN(user_id::text), MIN(type), MIN(title), MIN(message), bool_and(is_read), MIN(created_at), COUNT(*)
		          FROM notifications WHERE true`
		args := []any{}
		i := 1
		if uidParam != "" {
			query += ` AND user_id=$` + itoa(i)
			args = append(args, uidParam)
			i++
		}
		if typeParam != "" {
			query += ` AND type=$` + itoa(i)
			args = append(args, typeParam)
			i++
		}
		if unreadParam == "1" || unreadParam == "true" {
			query += ` AND is_read=false`
		}
		query += ` GROUP BY COALESCE(dispatch_id,id::text) ORDER BY MIN(created_at) DESC LIMIT $` + itoa(i) + ` OFFSET $` + itoa(i+1)
		args = append(args, limit, offset)

		rows, err := db.Query(ctx, query, args...)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memuat notifikasi")
			return
		}
		defer rows.Close()

		type adminNotif struct {
			ID        string    `json:"id"`
			UserID    string    `json:"user_id"`
			Type      string    `json:"type"`
			Title     string    `json:"title"`
			Message   string    `json:"message"`
			IsRead    bool      `json:"is_read"`
			CreatedAt time.Time `json:"created_at"`
			Count     int       `json:"count,omitempty"`
		}
		out := []adminNotif{}
		for rows.Next() {
			var n adminNotif
			var count int
			if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &n.IsRead, &n.CreatedAt, &count); err != nil {
				continue
			}
			n.Count = count
			out = append(out, n)
		}
		ok(w, out)
	}
}

// AdminCreateNotification: POST /api/admin/notifications
// { user_id: "exact-user-id"|"" , title, message, type: "SYSTEM" }
// user_id kosong = ke semua user. user_id terisi = hanya ke user itu.
func AdminCreateNotification(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in notifIn
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		in.Title = strings.TrimSpace(in.Title)
		in.Message = strings.TrimSpace(in.Message)
		if len(in.Title) < 1 || len(in.Title) > 120 || len(in.Message) < 1 || len(in.Message) > 2000 {
			fail(w, http.StatusBadRequest, "judul harus 1–120 karakter dan pesan 1–2000 karakter")
			return
		}
		// Admin hanya boleh set type SYSTEM; internal event pakai tipe spesifik.
		if in.Type != NotifSystem {
			in.Type = NotifSystem
		}
		var count int
		if in.UserID != "" {
			// Validate user exists to avoid silent no-op to a wrong ID.
			var exists bool
			err := db.QueryRow(r.Context(),
				`SELECT EXISTS(SELECT 1 FROM "user" WHERE id=$1)`, in.UserID).Scan(&exists)
			if err != nil || !exists {
				fail(w, http.StatusNotFound, "user tidak ditemukan")
				return
			}
			insertNotification(r.Context(), db, in.UserID, in.Type, in.Title, in.Message, in.OrderID, "")
			count = 1
		} else {
			insertNotificationToAll(r.Context(), db, in.Type, in.Title, in.Message, in.OrderID)
			var total int
			_ = db.QueryRow(r.Context(), `SELECT COUNT(*) FROM "user" WHERE role IN ('USER','SELLER')`).Scan(&total)
			count = total
		}
		audit(r.Context(), db, uidFromCtx(r.Context()), "ADMIN_NOTIF_CREATE", "", nil, in, ipOf(r))
		ok(w, map[string]any{"ok": true, "sent_to": count})
	}
}

// AdminDeleteNotification: DELETE /api/admin/notifications/{id}
func AdminDeleteNotification(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		tag, err := db.Exec(r.Context(),
			`DELETE FROM notifications WHERE id=$1`, id)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal menghapus")
			return
		}
		if tag.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "notifikasi tidak ditemukan")
			return
		}
		audit(r.Context(), db, uidFromCtx(r.Context()), "ADMIN_NOTIF_DELETE", id, nil, nil, ipOf(r))
		ok(w, map[string]any{"ok": true})
	}
}

// --- Event generators: dipanggil dari checkout/releaseEscrow/chat/etc ---

// genPurchaseNotification membuat notifikasi untuk pembeli setelah checkout OK.
func genPurchaseNotification(ctx context.Context, db DB, buyerID, orderID, title string) {
	insertNotification(ctx, db, buyerID, NotifPurchase, "Pembelian berhasil",
		title, &orderID, "")
}

// genSaleNotification memberitahu seller bahwa ordernya dibayar lunas.
func genSaleNotification(ctx context.Context, db DB, orderID, sellerID, title string) {
	insertNotification(ctx, db, sellerID, NotifSale, "Pesanan sudah dibayar",
		"Pembeli menyelesaikan pembayaran untuk \""+title+"\".", &orderID, "")
}

// genAffiliateCommissionNotification memberitahu affiliator bahwa komisinya
// sudah masuk dompet. Dipanggil DI DALAM transaksi pelepasan escrow, tepat di
// titik kredit komisi berhasil — jadi notif dan saldo sama-sama ada atau
// sama-sama tidak ada. Idempotensi dijamin pemanggil: blok ini hanya jalan saat
// INSERT affiliate_conversion ON CONFLICT (order_id) DO NOTHING menyisipkan
// baris baru, jadi tepat sekali per order walau escrow dilepas berulang.
func genAffiliateCommissionNotification(ctx context.Context, db execer, affiliatorID, orderID string, amount float64) {
	insertNotification(ctx, db, affiliatorID, NotifAffiliateCommission,
		"Komisi afiliasi masuk",
		"Komisi Rp"+trimZero(amount)+" dari order "+orderID+" sudah ditambahkan ke saldo dompet Anda.",
		nil, "")
}

// genOrderChatNotification memberitahu pihak lawan di order chat.
func genOrderChatNotification(ctx context.Context, db DB, orderID, recipientID, title, senderName string) {
	insertNotification(ctx, db, recipientID, NotifOrderChat,
		"Pesan baru: "+title, senderName+" mengirim pesan baru.", &orderID, "")
}

// genSystemNotification membuat notifikasi sistem ke satu user.
func genSystemNotification(ctx context.Context, db DB, userID, title, message string) {
	insertNotification(ctx, db, userID, NotifSystem, title, message, nil, "")
}

// insertNotificationToAdmins creates a SYSTEM notification for every admin user.
func insertNotificationToAdmins(ctx context.Context, db DB, title, message string) {
	rows, err := db.Query(ctx, `SELECT id FROM "user" WHERE role IN ('ADMIN','SUPER_ADMIN')`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return
		}
		insertNotification(ctx, db, id, NotifSystem, title, message, nil, "")
	}
}

// genDisputeResolvedNotification notifies buyer + seller when a dispute is resolved.
func genDisputeResolvedNotification(ctx context.Context, db DB, buyerID, storeID, orderID, status string) {
	switch status {
	case "BUYER_WINS":
		if buyerID != "" {
			go genSystemNotification(ctx, db, buyerID, "Sengketa Anda dikembalikan", "Sengketa pada pesanan "+orderID+" disetujuki. Dana telah dikembalikan.")
		}
		if storeID != "" {
			go genSystemNotification(ctx, db, storeID, "Sengketa disetujuti pembeli", "Sengketa pada pesanan "+orderID+" disetujui. Saldo toko Anda dikurangi.")
		}
	case "RESOLVED":
		if buyerID != "" {
			go genSystemNotification(ctx, db, buyerID, "Sengketa ditolak", "Sengketa Anda pada pesanan "+orderID+" ditolak.")
		}
		if storeID != "" {
			go genSystemNotification(ctx, db, storeID, "Sengketa ditolak", "Sengketa pada pesanan "+orderID+" ditolak. Toko Anda aman.")
		}
	default: // MANUAL_REVIEW
		if buyerID != "" {
			go genSystemNotification(ctx, db, buyerID, "Sengketa butuh review manual", "Sengketa Anda pada pesanan "+orderID+" memerlukan review manual admin.")
		}
		if storeID != "" {
			go genSystemNotification(ctx, db, storeID, "Sengketa butuh review manual", "Sengketa pada pesanan "+orderID+" memerlukan review manual admin.")
		}
	}
}
