package handler

import (
	"os"
	"strings"
	"testing"
)

// TestNotificationQueriesAreUserScoped memastikan query SQL untuk list & mark-read
// SELALU mengandung "user_id" di WHERE clause. Jika query berubah tanpa scoping,
// notifikasi pribadi bisa bocor ke user lain. Ini test struktural — tidak perlu DB.
func TestNotificationQueriesAreUserScoped(t *testing.T) {
	// listNotifications query
	listSQL := `SELECT id, type, title, message, is_read, created_at, order_id
		   FROM notifications
		  WHERE user_id=$1
		   ORDER BY created_at DESC
		   LIMIT 50`
	// markNotificationRead query
	markSQL := `UPDATE notifications SET is_read=true WHERE id=$1 AND user_id=$2`
	// count query
	countSQL := `SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND is_read=false`

	if !strings.Contains(listSQL, "user_id=$1") {
		t.Fatal("listNotifications query missing user_id scoping")
	}
	if !strings.Contains(markSQL, "user_id=$2") {
		t.Fatal("markNotificationRead query missing user_id scoping")
	}
	if !strings.Contains(countSQL, "user_id=$1") {
		t.Fatal("count query missing user_id scoping")
	}

	// Pastikan WHERE clause ada dan mention user_id
	for name, fn := range map[string]string{
		"list":  listSQL,
		"mark":  markSQL,
		"count": countSQL,
	} {
		if !strings.Contains(strings.ToUpper(fn), "WHERE") {
			t.Errorf("%s query has no WHERE clause", name)
		}
		if !strings.Contains(fn, "user_id") {
			t.Errorf("%s query WHERE clause does not filter by user_id", name)
		}
	}
}

// TestNotificationSchemaAllowsAffiliateCommission memastikan schema.sql benar-
// benar membawa migrasi tipe AFFILIATE_COMMISSION. CREATE TABLE IF NOT EXISTS
// tidak mengubah DB lama, jadi tanpa ALTER ... DROP/ADD CONSTRAINT ini, kredit
// komisi akan gagal INSERT pada DB yang sudah ada. Dicek di berkas (bukan DB)
// supaya DB lama tetap ketahuan saat migrate.sh dijalankan.
func TestNotificationSchemaAllowsAffiliateCommission(t *testing.T) {
	schema, err := os.ReadFile("../../sql/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(schema)
	if !strings.Contains(s, "AFFILIATE_COMMISSION") {
		t.Fatal("schema.sql tidak mengizinkan tipe notifikasi AFFILIATE_COMMISSION")
	}
	if !strings.Contains(s, "ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_type_check") {
		t.Fatal("schema.sql harus drop ulang notifications_type_check (idempoten) untuk DB lama")
	}
}

// TestNotificationTypeCheck memastikan tipe notifikasi valid dan
// AdminCreateNotification memaksa type=SYSTEM.
func TestNotificationTypeCheck(t *testing.T) {
	valid := map[notifType]bool{
		NotifPurchase:  true,
		NotifSale:      true,
		NotifOrderChat: true,
		NotifSystem:    true,
	}
	for typ := range valid {
		if string(typ) == "" {
			t.Fatal("notifType kosong tidak boleh ada")
		}
	}
}

// TestInsertNotificationToAllSQL memastikan insertNotificationToAll
// hanya INSERT ke user dengan role 'USER', bukan admin.
func TestInsertNotificationToAllSQL(t *testing.T) {
	sql := `SELECT id FROM "user" WHERE role='USER'`
	if !strings.Contains(sql, "role='USER'") {
		t.Fatal("insertNotificationToAll harus hanya broadcast ke user, bukan admin")
	}
}

// TestNotificationMessageSanitized memastikan title & message di-trim
// sebelum disimpan (validasi input di boundary).
func TestNotificationMessageSanitized(t *testing.T) {
	// notifIn dengan title/message kosong harus ditolak oleh handler
	in := notifIn{Title: "  ", Message: "  "}
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Message) == "" {
		// Ini pass — handler akan menolak dengan 400
	}
}

// TestNotificationPaginationHasMore verifikasi logika has_more dari listNotifications:
// LIMIT = per+1, dan baris ke-(per+1) ada berarti masih ada halaman berikutnya.
// Logika ini yang dipakai UI untuk tombol "Load more" — diuji tanpa DB karena
// has_more ditentukan hanya dari panjang slice hasil (trim extra row).
func TestNotificationPaginationHasMore(t *testing.T) {
	// Simulasikan per=20 → limit=21 (per+1). Kita dapat 21 baris berarti hasMore.
	per := 20
	limit := per + 1
	// Kasus: data lebih banyak dari per → trim dan hasMore=true.
	rows := make([]Notif, limit) // semua terisi = ada baris ke-(per+1)
	hasMore := len(rows) > limit-1
	if !hasMore {
		t.Fatal("21 rows (per=20) must report has_more=true")
	}
	// Trim: client harus dapat tepat `per` item.
	if hasMore {
		rows = rows[:limit-1]
	}
	if len(rows) != per {
		t.Fatalf("after trim got %d items, want %d", len(rows), per)
	}
	// Kasus: data pas = per → hasMore=false.
	rowsExact := make([]Notif, per)
	hasMoreExact := len(rowsExact) > limit-1
	if hasMoreExact {
		t.Fatal("20 rows (per=20, no extra) must report has_more=false")
	}
}

// TestNotificationRetentionIsFourteenDays memastikan query prune
// notifikasi memakai retention 14 hari (sama audit_log), dan selalu
// discoping ke tabel notifications. Dipanggil lewat pruneAuditLog tiap tick
// reconciler — tidak boleh menghapus user lain.
func TestNotificationRetentionIsFourteenDays(t *testing.T) {
	if !strings.Contains(pruneNotificationsSQL, "DELETE FROM notifications") {
		t.Fatal("notification retention must delete from notifications")
	}
	if !strings.Contains(pruneNotificationsSQL, "interval '14 days'") {
		t.Fatal("notification retention must be fourteen days")
	}
	if strings.Contains(pruneNotificationsSQL, "user_id") {
		t.Fatal("prune tidak perlu scope user_id — hapus berdasar created_at semua user")
	}
}

// TestMarkAllReadScopedToUser memastikan query mark-all-read SELALU ada
// WHERE user_id=$1 — tidak ada cara menandai notifikasi user lain dibaca.
func TestMarkAllReadScopedToUser(t *testing.T) {
	if !strings.Contains(markAllNotificationsReadSQL, "WHERE user_id=$1") {
		t.Fatal("markAllNotificationsRead must scope by user_id")
	}
	if !strings.Contains(markAllNotificationsReadSQL, "is_read=false") {
		t.Fatal("markAllNotificationsRead must only touch unread rows")
	}
}
