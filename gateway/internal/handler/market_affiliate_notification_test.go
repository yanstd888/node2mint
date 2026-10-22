package handler

import (
	"context"
	"os"
	"strings"
	"testing"

	"ai-sell/gateway/internal/store"

	"github.com/jackc/pgx/v5/pgconn"
)

// captureExecer menangkap SQL + args tanpa DB. Cukup untuk membuktikan
// genAffiliateCommissionNotification menulis baris dengan tipe & payload benar.
type captureExecer struct {
	sqls []string
	args [][]any
}

func (c *captureExecer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	c.sqls = append(c.sqls, sql)
	c.args = append(c.args, args)
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

// TestAffiliateCommissionNotificationPayload: tipe harus AFFILIATE_COMMISSION
// (bukan SYSTEM), dan nominal komisi harus muncul di pesan — ini yang dibaca
// affiliator di dashboard, bukan sekadar baris ada.
func TestAffiliateCommissionNotificationPayload(t *testing.T) {
	c := &captureExecer{}
	genAffiliateCommissionNotification(context.Background(), c, "aff-1", "order-1", 18000)

	if len(c.sqls) != 1 {
		t.Fatalf("ingin 1 insert, dapat %d", len(c.sqls))
	}
	if !strings.Contains(c.sqls[0], "INSERT INTO notifications") {
		t.Fatalf("SQL bukan insert notifications: %s", c.sqls[0])
	}
	// Argumen sesuai urutan kolom: user_id, type, title, message, order_id, dispatch_id.
	if got := c.args[0][1]; got != string(NotifAffiliateCommission) {
		t.Fatalf("type notif = %v, mau %s", got, NotifAffiliateCommission)
	}
	if NotifAffiliateCommission == NotifSystem {
		t.Fatal("tipe komisi afiliasi tidak boleh sama dengan SYSTEM")
	}
	msg, _ := c.args[0][3].(string)
	if !strings.Contains(msg, "18000") {
		t.Fatalf("pesan tidak memuat nominal komisi: %q", msg)
	}
	if !strings.Contains(msg, "order-1") {
		t.Fatalf("pesan tidak memuat order_id: %q", msg)
	}
	if got := c.args[0][0]; got != "aff-1" {
		t.Fatalf("user_id = %v, mau aff-1 (affiliator)", got)
	}
}

// TestReleaseEscrowCreditsAffiliateNotification: bukti jalur uang nyata —
// pelepasan escrow order berafiliasi menulis tepat SATU notifikasi komisi ke
// affiliator dengan nominal = affiliate_commission_amount, dan pelepasan ulang
// (idempoten) TIDAK menambah notif kedua. Dilewati bila DATABASE_URL tidak diset.
func TestReleaseEscrowCreditsAffiliateNotification(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji notif komisi afiliasi")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	const commission = 18000
	_, affiliator, orderID := seedAffiliateOrderFixture(t, ctx, db, commission)

	count := func() int {
		var n int
		if err := db.QueryRow(ctx,
			`SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND type='AFFILIATE_COMMISSION'`,
			affiliator).Scan(&n); err != nil {
			t.Fatalf("hitung notif: %v", err)
		}
		return n
	}

	if count() != 0 {
		t.Fatal("fixture sudah punya notif komisi sebelum escrow dilepas")
	}
	// 1) Pelepasan pertama: kredit + satu notif.
	if _, reason, ok := releaseEscrowAuto(ctx, db, orderID); !ok {
		t.Fatalf("releaseEscrow gagal: %s", reason)
	}
	if n := count(); n != 1 {
		t.Fatalf("setelah release: %d notif komisi, mau 1", n)
	}
	var msg string
	if err := db.QueryRow(ctx,
		`SELECT message FROM notifications WHERE user_id=$1 AND type='AFFILIATE_COMMISSION'`,
		affiliator).Scan(&msg); err != nil {
		t.Fatalf("baca pesan: %v", err)
	}
	if !strings.Contains(msg, "18000") {
		t.Fatalf("pesan komisi tidak memuat nominal: %q", msg)
	}
	// 2) Pelepasan ulang (order sudah COMPLETED) ditolak dan TIDAK menambah notif.
	if _, _, ok := releaseEscrowAuto(ctx, db, orderID); ok {
		t.Fatal("releaseEscrow kedua seharusnya ditolak untuk order COMPLETED")
	}
	if n := count(); n != 1 {
		t.Fatalf("setelah release ulang: %d notif komisi, mau tetap 1 (idempoten)", n)
	}
}

// seedAffiliateOrderFixture: affiliator + link + seller/toko + produk + order
// PAID berafiliasi (komisi sudah disnapshot di order, persis seusai checkout).
// Bersih dibersihkan lewat CASCADE saat user fixture dihapus; order dihapus
// lebih dulu (lewat buyer) karena market_order.product_id tidak bercascade.
func seedAffiliateOrderFixture(t *testing.T, ctx context.Context, db DB, commission float64) (buyer, affiliator, orderID string) {
	t.Helper()
	var seller string
	if err := db.QueryRow(ctx,
		`INSERT INTO "user" (email, password_hash, wallet_balance) VALUES ($1,'x',0) RETURNING id`,
		"affnotif-seller-"+randCode(8)+"@example.invalid").Scan(&seller); err != nil {
		t.Fatalf("seed seller: %v", err)
	}
	t.Cleanup(func() { deleteFixture(t, ctx, db, seller) })

	var storeID string
	if err := db.QueryRow(ctx,
		`INSERT INTO market_store (user_id, name, wallet_balance) VALUES ($1,'Aff Notif',$2) RETURNING id`,
		seller, 0).Scan(&storeID); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	var product string
	if err := db.QueryRow(ctx,
		`INSERT INTO market_product (seller_id, store_id, title, price, stock, is_active)
		 VALUES ($1,$2,'Aff Notif',100,0,true) RETURNING id`, seller, storeID).Scan(&product); err != nil {
		t.Fatalf("seed produk: %v", err)
	}

	if err := db.QueryRow(ctx,
		`INSERT INTO "user" (email, password_hash, wallet_balance) VALUES ($1,'x',0) RETURNING id`,
		"affnotif-affiliate-"+randCode(8)+"@example.invalid").Scan(&affiliator); err != nil {
		t.Fatalf("seed affiliator: %v", err)
	}
	t.Cleanup(func() { deleteFixture(t, ctx, db, affiliator) })

	code := "AFF" + randCode(5)
	if _, err := db.Exec(ctx,
		`INSERT INTO affiliate_link (product_id, user_id, code) VALUES ($1,$2,$3)`,
		product, affiliator, code); err != nil {
		t.Fatalf("seed affiliate_link: %v", err)
	}

	buyer = seedUser(t, ctx, db)
	t.Cleanup(func() { deleteFixture(t, ctx, db, buyer) })

	if err := db.QueryRow(ctx,
		`INSERT INTO market_order (buyer_id, product_id, store_id, qty, total_price, subtotal,
		                           fee_amount, seller_payout, affiliate_code, affiliate_commission_amount,
		                           status, process_deadline, received_at)
		 VALUES ($1,$2,$3,1,100,100,10,900,$4,$5,'PAID', now() + interval '1 hour', now())
		 RETURNING id`,
		buyer, product, storeID, code, commission).Scan(&orderID); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	return buyer, affiliator, orderID
}
