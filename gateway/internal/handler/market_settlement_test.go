package handler

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"ai-sell/gateway/internal/store"
)

func TestAutoReleaseEligible(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	justBefore := now.Add(-48*time.Hour + time.Nanosecond)
	atThreshold := now.Add(-48 * time.Hour)

	cases := []struct {
		name     string
		status   string
		received *time.Time
		disputed bool
		want     bool
	}{
		{"paid delivery reaches 48 hours", "PAID", &atThreshold, false, true},
		{"processing delivery reaches 48 hours", "PROCESSING", &atThreshold, false, true},
		{"delivery is still inside hold", "PAID", &justBefore, false, false},
		{"missing delivery timestamp stays frozen", "PAID", nil, false, false},
		{"open dispute stays frozen", "PAID", &atThreshold, true, false},
		{"completed order cannot be released twice", "COMPLETED", &atThreshold, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := autoReleaseEligible(tc.status, tc.received, tc.disputed, now); got != tc.want {
				t.Fatalf("autoReleaseEligible(%q, %v, disputed=%t)=%t, want %t", tc.status, tc.received, tc.disputed, got, tc.want)
			}
		})
	}
}

func TestSellerMayMarkDelivery(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	before := now.Add(time.Hour)
	after := now.Add(-time.Hour)

	for _, tc := range []struct {
		status    string
		isInstant bool
		deadline  *time.Time
		want      bool
	}{
		{"PAID", false, &before, true},
		{"PAID", false, &after, false},      // lewat batas kirim 24h
		{"PAID", false, &now, false},        // tepat di batas = sudah lewat
		{"PAID", false, nil, true},          // order lama tanpa deadline
		{"PROCESSING", false, &after, true}, // sudah dikirim, tidak bisa dibatalkan lagi
		{"PAID", true, &before, false},      // instan: tidak ada aksi seller
		{"DISPUTED", false, &before, false},
		{"COMPLETED", false, &before, false},
		{"CANCELLED", false, &before, false},
	} {
		if got := sellerMayMarkDelivery(tc.status, tc.isInstant, tc.deadline, now); got != tc.want {
			t.Fatalf("sellerMayMarkDelivery(%q, instant=%t, deadline=%v)=%t, want %t",
				tc.status, tc.isInstant, tc.deadline, got, tc.want)
		}
	}
}

// TestNextOrderStatus: seller-wins harus menghasilkan order PAID yang bisa
// dilepas escrow-nya, buyer-wins CANCELLED, dan manual_review tidak mengubah
// apa pun (order tetap DISPUTED = escrow tetap tertahan).
func TestNextOrderStatus(t *testing.T) {
	cases := []struct {
		dispute, order, want string
	}{
		{"BUYER_WINS", "DISPUTED", "CANCELLED"},
		{"BUYER_WINS", "PROCESSING", "CANCELLED"},
		{"RESOLVED", "DISPUTED", "PAID"},
		{"RESOLVED", "COMPLETED", ""}, // sudah cair sebelum sengketa: jangan diubah
		{"MANUAL_REVIEW", "DISPUTED", ""},
	}
	for _, c := range cases {
		if got := nextOrderStatus(c.dispute, c.order); got != c.want {
			t.Fatalf("nextOrderStatus(%q, %q)=%q, want %q", c.dispute, c.order, got, c.want)
		}
	}
}

func TestDisputeOrderHadSettledEscrow(t *testing.T) {
	completed := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name      string
		status    string
		completed *time.Time
		want      bool
	}{
		{"disputed from completed keeps payout settled", "DISPUTED", &completed, true},
		{"ordinary disputed escrow is still held", "DISPUTED", nil, false},
		{"completed status is settled", "COMPLETED", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := disputeOrderHadSettledEscrow(tc.status, tc.completed); got != tc.want {
				t.Fatalf("disputeOrderHadSettledEscrow(%q,%v)=%t, want %t", tc.status, tc.completed, got, tc.want)
			}
		})
	}
}

// TestEscrowDisputeMoneyPaths: uji jalur uang yang tidak bisa dibuktikan tanpa
// DB — hold sengketa benar-benar menahan pelepasan, refund buyer_wins
// mengembalikan stok + melepas barang instan, seller_wins melanjutkan order ke
// COMPLETED, dan order tanpa toko ditolak (bukan ditutup diam-diam).
// Dilewati bila DATABASE_URL tidak diset. Tidak ada reset DB: hanya baris
// fixture milik tes ini yang dibuat, lalu dihapus via DELETE (CASCADE).
func TestEscrowDisputeMoneyPaths(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL tidak diset — lewati uji jalur uang escrow")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	// Ditutup lewat t.Cleanup (bukan defer) supaya pembersihan fixture — yang
	// juga t.Cleanup dan berjalan LIFO — dapat memakai pool yang masih hidup.
	t.Cleanup(func() { db.Close() })

	// 1) Sengketa OPEN menahan pelepasan escrow, walau saldo toko cukup.
	buyer, _, orderID := seedEscrowFixture(t, ctx, db, true, 100)
	if _, _, ok := releaseEscrowAuto(ctx, db, orderID); ok {
		t.Fatal("releaseEscrow harus menolak order yang sedang disengketakan")
	}
	if st, bal := orderState(t, ctx, db, orderID, buyer); st != "DISPUTED" || bal != 0 {
		t.Fatalf("escrow bocor saat sengketa: status=%s saldo_pembeli=%v", st, bal)
	}

	// 2) buyer_wins: refund penuh dari saldo toko + stok & barang instan kembali.
	//    Urutannya sama dengan ResolveDispute (refund lalu restock, satu transaksi).
	if _, err := db.Exec(ctx, `UPDATE market_dispute SET status='BUYER_WINS' WHERE order_id=$1`, orderID); err != nil {
		t.Fatalf("set dispute: %v", err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	var storeID string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(store_id::text,'') FROM market_order WHERE id=$1`, orderID).Scan(&storeID); err != nil {
		t.Fatalf("baca store order: %v", err)
	}
	refunded, err := doBuyerWinsRefund(ctx, tx, storeID, buyer, 100, orderID)
	if err != nil || !refunded {
		t.Fatalf("refund buyer_wins: refunded=%t err=%v", refunded, err)
	}
	if err := restockOrder(ctx, tx, orderID); err != nil {
		t.Fatalf("restock: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	var stock, siap int
	if err := db.QueryRow(ctx,
		`SELECT (SELECT stock FROM market_product p JOIN market_order o ON o.product_id=p.id WHERE o.id=$1),
		        (SELECT COUNT(*) FROM market_product_item WHERE order_id=$1)`, orderID).Scan(&stock, &siap); err != nil {
		t.Fatalf("baca stok: %v", err)
	}
	if stock != 2 || siap != 0 {
		t.Fatalf("setelah buyer_wins: stok=%d barang_masih_terikat=%d, mau 2/0 (instan wajib dilepas)", stock, siap)
	}
	var storeBal, buyerBal float64
	if err := db.QueryRow(ctx,
		`SELECT (SELECT wallet_balance FROM market_store WHERE id=$1), (SELECT wallet_balance FROM "user" WHERE id=$2)`,
		storeID, buyer).Scan(&storeBal, &buyerBal); err != nil {
		t.Fatalf("baca saldo: %v", err)
	}
	if storeBal != 0 || buyerBal != 100 {
		t.Fatalf("saldo buyer_wins: toko=%v pembeli=%v, mau 0/100", storeBal, buyerBal)
	}

	// 3) seller_wins: sengketa RESOLVED melanjutkan order ke COMPLETED dan
	//    escrow benar-benar dibayar ke toko.
	buyer2, store2, order2 := seedEscrowFixture(t, ctx, db, false, 0)
	tx2, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx2.Exec(ctx, `UPDATE market_dispute SET status='RESOLVED' WHERE order_id=$1`, order2); err != nil {
		t.Fatalf("set dispute: %v", err)
	}
	if _, err := tx2.Exec(ctx, `UPDATE market_order SET status='PAID' WHERE id=$1 AND status='DISPUTED'`, order2); err != nil {
		t.Fatalf("resume order: %v", err)
	}
	res, reason, fine := releaseEscrowTx(ctx, tx2, order2)
	if !fine {
		t.Fatalf("releaseEscrowTx setelah seller_wins gagal: %s", reason)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if res.SellerPayout != 100 {
		t.Fatalf("payout seller_wins=%v, mau 100", res.SellerPayout)
	}
	if st, _ := orderState(t, ctx, db, order2, buyer2); st != "COMPLETED" {
		t.Fatalf("order seller_wins=%s, mau COMPLETED", st)
	}
	if err := db.QueryRow(ctx, `SELECT wallet_balance FROM market_store WHERE id=$1`, store2).Scan(&storeBal); err != nil {
		t.Fatalf("baca saldo toko: %v", err)
	}
	if storeBal != 100 {
		t.Fatalf("saldo toko setelah seller_wins=%v, mau 100", storeBal)
	}

	// 4) Tanpa toko: ditolak, order TIDAK ditutup COMPLETED. Produk seed lama
	// tanpa store_id adalah satu-satunya cara order mencapai keadaan ini.
	var product3, order3 string
	if err := db.QueryRow(ctx,
		`INSERT INTO market_product (seller_id, title, price, stock, is_active)
		 VALUES ($1,'Tanpa Toko',50,0,true) RETURNING id`, buyer2).Scan(&product3); err != nil {
		t.Fatalf("seed produk tanpa toko: %v", err)
	}
	if err := db.QueryRow(ctx,
		`INSERT INTO market_order (buyer_id, product_id, qty, total_price, subtotal, seller_payout, status)
		 VALUES ($1,$2,1,50,50,45,'PAID') RETURNING id`, buyer2, product3).Scan(&order3); err != nil {
		t.Fatalf("seed order tanpa toko: %v", err)
	}
	if _, _, ok := releaseEscrowAuto(ctx, db, order3); ok {
		t.Fatal("releaseEscrow tanpa toko harus ditolak, bukan menutup order")
	}
	if st, _ := orderState(t, ctx, db, order3, buyer2); st == "COMPLETED" {
		t.Fatal("order tanpa toko tidak boleh ditutup COMPLETED — dana menggantung tanpa penerima")
	}

	// 5) Query pengunci ResolveDispute harus jalan terhadap baris nyata, bukan
	//    cuma lolos parse: `FOR UPDATE OF d, o` pada JOIN dua tabel.
	tx5, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx5.Rollback(ctx)
	var lockedDispute string
	if err := tx5.QueryRow(ctx,
		`SELECT d.id::text FROM market_dispute d JOIN market_order o ON o.id=d.order_id
		  WHERE d.id=$1 FOR UPDATE OF d, o`, disputeID(t, ctx, db, order2)).Scan(&lockedDispute); err != nil {
		t.Fatalf("SELECT ... FOR UPDATE OF d, o gagal pada baris nyata: %v", err)
	}

	// 6) cancelOrder (jalur yang dijalankan reconciler tiap menit) memakai
	//    restockOrder yang sama: stok +1 dan barang instan dilepas, DAN
	//    sengketa terbuka memblokirnya.
	buyer4, _, order4 := seedEscrowFixture(t, ctx, db, true, 100)
	if code, _ := cancelOrder(ctx, db, order4, "", true, "", "", ""); code != http.StatusConflict {
		t.Fatalf("cancelOrder atas order bersengketa harus 409, dapat %d", code)
	}
	if _, err := db.Exec(ctx, `DELETE FROM market_dispute WHERE order_id=$1`, order4); err != nil {
		t.Fatalf("hapus sengketa: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE market_order SET status='PAID' WHERE id=$1`, order4); err != nil {
		t.Fatalf("order ke PAID: %v", err)
	}
	if code, reason := cancelOrder(ctx, db, order4, "", true, "Auto-cancel: seller belum mengirim", "", ""); code != 0 {
		t.Fatalf("cancelOrder gagal: %d %s", code, reason)
	}
	var stock4, tied4 int
	if err := db.QueryRow(ctx,
		`SELECT (SELECT stock FROM market_product p JOIN market_order o ON o.product_id=p.id WHERE o.id=$1),
		        (SELECT COUNT(*) FROM market_product_item WHERE order_id=$1)`, order4).Scan(&stock4, &tied4); err != nil {
		t.Fatalf("baca stok: %v", err)
	}
	if stock4 != 2 || tied4 != 0 {
		t.Fatalf("setelah cancelOrder: stok=%d barang_terikat=%d, mau 2/0", stock4, tied4)
	}
	if st, bal := orderState(t, ctx, db, order4, buyer4); st != "CANCELLED" || bal != 100 {
		t.Fatalf("cancelOrder: status=%s saldo_pembeli=%v, mau CANCELLED/100", st, bal)
	}
}

// disputeID: id sengketa milik satu order.
func disputeID(t *testing.T, ctx context.Context, db DB, orderID string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(ctx, `SELECT id FROM market_dispute WHERE order_id=$1`, orderID).Scan(&id); err != nil {
		t.Fatalf("baca dispute: %v", err)
	}
	return id
}

// seedEscrowFixture: seller + toko (saldo awal) + pembeli + produk instan/
// non-instan + order PAID dalam sengketa OPEN. Kedua user dibersihkan lewat
// t.Cleanup — CASCADE menghapus toko, produk, order, sengketa, dan mutasinya.
func seedEscrowFixture(t *testing.T, ctx context.Context, db DB, instant bool, storeBalance float64) (buyer, storeID, orderID string) {
	t.Helper()
	var seller string
	if err := db.QueryRow(ctx,
		`INSERT INTO "user" (email, password_hash, wallet_balance) VALUES ($1,'x',0) RETURNING id`,
		"escrow-seller-"+randCode(8)+"@example.invalid").Scan(&seller); err != nil {
		t.Fatalf("seed seller: %v", err)
	}
	t.Cleanup(func() { deleteFixture(t, ctx, db, seller) })
	if err := db.QueryRow(ctx,
		`INSERT INTO market_store (user_id, name, wallet_balance) VALUES ($1,'Escrow Test',$2) RETURNING id`,
		seller, storeBalance).Scan(&storeID); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	buyer = seedUser(t, ctx, db)
	t.Cleanup(func() { deleteFixture(t, ctx, db, buyer) })

	product := mustProductTo(t, ctx, db, seller, storeID, instant)
	if instant {
		if _, err := db.Exec(ctx,
			`INSERT INTO market_product_item (product_id, content) VALUES ($1,'kode-1'),($1,'kode-2')`, product); err != nil {
			t.Fatalf("seed barang instan: %v", err)
		}
	}
	// Order menahan 2 unit; stok sudah dipotong (0) dan barang instan terikat —
	// persis keadaan setelah checkout, jadi restock bisa diuji apa adanya.
	if err := db.QueryRow(ctx,
		`INSERT INTO market_order (buyer_id, product_id, store_id, qty, total_price, subtotal, seller_payout,
		                           status, received_at, process_deadline)
		 VALUES ($1,$2,$3,2,100,100,100,'PAID', now(), now() + interval '1 hour') RETURNING id`,
		buyer, product, storeID).Scan(&orderID); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	if instant {
		if _, err := db.Exec(ctx, `UPDATE market_product_item SET order_id=$1 WHERE product_id=$2`, orderID, product); err != nil {
			t.Fatalf("ikat barang instan: %v", err)
		}
	}
	if _, err := db.Exec(ctx,
		`UPDATE market_order SET status='DISPUTED' WHERE id=$1`, orderID); err != nil {
		t.Fatalf("order ke DISPUTED: %v", err)
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO market_dispute (order_id, buyer_id, store_id, reason)
		 VALUES ($1,$2,$3,'barang tidak sesuai deskripsi toko')`, orderID, buyer, storeID); err != nil {
		t.Fatalf("seed dispute: %v", err)
	}
	return buyer, storeID, orderID
}

func mustProductTo(t *testing.T, ctx context.Context, db DB, seller, storeID string, instant bool) string {
	t.Helper()
	var id string
	if err := db.QueryRow(ctx,
		`INSERT INTO market_product (seller_id, store_id, title, price, stock, is_active, is_instant)
		 VALUES ($1,$2,'Escrow Test',50,0,true,$3) RETURNING id`, seller, storeID, instant).Scan(&id); err != nil {
		t.Fatalf("seed produk: %v", err)
	}
	return id
}

// orderState: status order + saldo dompet pembeli.
func orderState(t *testing.T, ctx context.Context, db DB, orderID, buyer string) (status string, buyerBalance float64) {
	t.Helper()
	if err := db.QueryRow(ctx,
		`SELECT o.status, (SELECT wallet_balance FROM "user" WHERE id=o.buyer_id)
		   FROM market_order o WHERE o.id=$1`, orderID).Scan(&status, &buyerBalance); err != nil {
		t.Fatalf("baca order: %v", err)
	}
	return status, buyerBalance
}
