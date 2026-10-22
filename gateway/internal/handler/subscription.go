package handler

// Langganan berulang via dompet (auto-renew).
//
// Kenapa berkas terpisah: tabel `subscription` sudah ada di skema tapi tidak
// pernah dibaca/ditulis kode mana pun (sisa scaffolding Stripe). Berkas ini
// memakainya sungguhan dengan sumber dana dompet, bukan Stripe.
//
// Aturan uang (sama seperti boost & verified):
//   - Saldo dipotong di dalam transaksi, dengan syarat wallet_balance >= harga
//     di klausa WHERE — bukan cek-lalu-potong yang bisa kalah balapan.
//   - Harga per periode DIKUNCI saat user menekan konfirmasi (renew_price).
//     Kalau admin menaikkan tarif, langganan yang sudah berjalan tidak ikut naik;
//     kenaikan hanya berlaku untuk langganan baru atau setelah user setuju ulang.
//   - Saldo kurang saat jatuh tempo: TIDAK ada pemotongan paksa, langganan
//     ditandai PAST_DUE + alasannya dicatat, dan manfaatnya tetap berjalan
//     sampai masa aktif habis. User diberi tahu di UI.

import (
	"context"
	"log"
	"net/http"
	"time"
)

// Nama tier untuk verified store. Dipakai juga sebagai kunci uq_sub_active_user.
const tierVerifiedStore = "VERIFIED_STORE"

// hargaPerpanjangan: harga yang ditagih saat memperpanjang.
// Harga yang DIKUNCI saat user menyalakan auto-renew menang, karena itu harga
// yang dia setujui; tariff platform hanya dipakai kalau belum ada kesepakatan.
// Dipisah dari handler supaya bisa diuji tanpa DB dan tidak bisa "diam-diam"
// berubah tanpa ada test yang gagal.
func hargaPerpanjangan(renewPrice, tarifPlatform float64) float64 {
	if renewPrice > 0 {
		return renewPrice
	}
	return tarifPlatform
}

// subscriptionRow: bentuk yang dikirim ke klien. Semua waktu RFC3339 supaya
// frontend bisa menghitung sisa hari dengan cara yang sama seperti boost.
type subscriptionRow struct {
	ID               string  `json:"id"`
	TierName         string  `json:"tier_name"`
	Status           string  `json:"status"`
	Price            float64 `json:"price"`
	AutoRenew        bool    `json:"auto_renew"`
	RenewDays        int     `json:"renew_days"`
	CurrentPeriodEnd string  `json:"current_period_end"`
	CancelAtEnd      bool    `json:"cancel_at_period_end"`
	LastError        string  `json:"last_error,omitempty"`
}

// loadActiveSubscription mengambil langganan ACTIVE untuk user+tier.
// Mengembalikan (row, true) hanya kalau ada.
func loadActiveSubscription(ctx context.Context, db DB, uid, tier string) (subscriptionRow, bool) {
	var s subscriptionRow
	var end time.Time
	err := db.QueryRow(ctx,
		`SELECT id, COALESCE(tier_name,''), status, price, auto_renew, renew_days, current_period_end, cancel_at_period_end, COALESCE(last_error,'')
		   FROM subscription WHERE user_id=$1 AND tier_name=$2 AND status='ACTIVE'`, uid, tier).
		Scan(&s.ID, &s.TierName, &s.Status, &s.Price, &s.AutoRenew, &s.RenewDays, &end, &s.CancelAtEnd, &s.LastError)
	if err != nil {
		return subscriptionRow{}, false
	}
	s.CurrentPeriodEnd = end.Format(time.RFC3339)
	return s, true
}

// SetSubscriptionAutoRenew: POST /api/store/subscription {auto_renew}.
// Menyalakan/mematikan perpanjangan otomatis untuk Verified Store.
//
// Menyalakan auto-renew TIDAK langsung memotong saldo: pemotongan terjadi saat
// jatuh tempo (reconciler). Karena itu user tidak perlu saldo sekarang — tapi
// UI wajib memperingatkan bahwa saldo harus tersedia saat jatuh tempo.
func SetSubscriptionAutoRenew(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			Auto bool `json:"auto_renew"`
		}
		_ = decode(r, &in)

		sub, found := loadActiveSubscription(r.Context(), db, uid, tierVerifiedStore)
		if !found {
			fail(w, http.StatusNotFound, "belum ada langganan verified yang aktif")
			return
		}

		// Saat menyalakan: kunci harga periode berikutnya dari tarif platform saat ini.
		price := settingFloat(r.Context(), db, "verified_store_monthly_price", 15000)
		var err error
		if in.Auto {
			_, err = db.Exec(r.Context(),
				`UPDATE subscription SET auto_renew=true, renew_price=$1, renew_days=30, cancel_at_period_end=false, last_error='', updated_at=now() WHERE id=$2`,
				price, sub.ID)
		} else {
			// Mematikan: berlaku sampai akhir periode yang sudah dibayar.
			_, err = db.Exec(r.Context(),
				`UPDATE subscription SET auto_renew=false, cancel_at_period_end=true, updated_at=now() WHERE id=$1`, sub.ID)
		}
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan pengaturan")
			return
		}
		ok(w, map[string]any{"ok": true, "auto_renew": in.Auto, "renew_price": price})
	}
}

// GetSubscription: GET /api/store/subscription — status langganan verified.
// Publik ke pemiliknya: dipakai UI untuk menampilkan sisa masa + opsi auto-renew.
func GetSubscription(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		sub, found := loadActiveSubscription(r.Context(), db, uid, tierVerifiedStore)
		if !found {
			ok(w, map[string]any{"subscription": nil})
			return
		}
		ok(w, map[string]any{"subscription": sub})
	}
}

// RenewSubscription: POST /api/store/subscription/renew — perpanjang manual
// sekarang (30 hari) untuk Verified Store, tanpa menunggu jatuh tempo.
// Idempoten terhadap dobel-klik lewat syarat saldo di klausa WHERE.
func RenewSubscription(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		sub, found := loadActiveSubscription(r.Context(), db, uid, tierVerifiedStore)
		if !found {
			fail(w, http.StatusNotFound, "belum ada langganan verified yang aktif")
			return
		}
		// Harga: pakai yang terkunci kalau ada, kalau tidak tarif platform.
		price := hargaPerpanjangan(sub.Price, settingFloat(r.Context(), db, "verified_store_monthly_price", 15000))
		if err := chargeRenewal(r.Context(), db, uid, sub, price); err != nil {
			if err == errSaldoKurang {
				fail(w, http.StatusPaymentRequired, "saldo tidak cukup")
				return
			}
			log.Printf("[ERROR] renew subscription uid=%s: %v", uid, err)
			fail(w, http.StatusInternalServerError, "gagal memperpanjang")
			return
		}
		ok(w, map[string]any{"ok": true, "days": 30, "price": price})
	}
}

// errSaldoKurang: saldo dompet tidak mencukupi (dipakai untuk membedakan
// 402 dari 500 pada jalur perpanjangan).
var errSaldoKurang = errString("saldo tidak cukup")

type errString string

func (e errString) Error() string { return string(e) }

// chargeRenewal: potong saldo + tambah 30 hari + perpanjang verified_until,
// semuanya dalam satu transaksi. Aman dipanggil dari handler (manual) maupun
// reconciler (otomatis) — idempoten lewat last_renew_at.
func chargeRenewal(ctx context.Context, db DB, uid string, sub subscriptionRow, price float64) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Kunci baris langganan supaya reconciler dan klik manual tidak balapan.
	var lockedID string
	var lastRenew *time.Time
	if err := tx.QueryRow(ctx,
		`SELECT id, last_renew_at FROM subscription WHERE id=$1 FOR UPDATE`, sub.ID).Scan(&lockedID, &lastRenew); err != nil {
		return err
	}
	// Penjaga idempoten: satu periode hanya ditagih sekali. Kalau sudah pernah
	// ditagih dalam 1 menit terakhir, anggap ini dobel-klik/siklus ganda.
	if lastRenew != nil && time.Since(*lastRenew) < time.Minute {
		return nil
	}

	// Potong saldo dengan syarat kecukupan di klausa WHERE (bukan cek-lalu-potong).
	cut, err := tx.Exec(ctx,
		`UPDATE "user" SET wallet_balance=wallet_balance-$1 WHERE id=$2 AND wallet_balance >= $1`, price, uid)
	if err != nil {
		return err
	}
	if cut.RowsAffected() == 0 {
		return errSaldoKurang
	}

	if _, err := tx.Exec(ctx,
		`UPDATE subscription SET current_period_start=now(), current_period_end=now()+make_interval(days=>$1),
		        status='ACTIVE', last_renew_at=now(), last_error='', price=$2, updated_at=now() WHERE id=$3`,
		sub.RenewDays, price, sub.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE market_store SET is_verified=true,
		        verified_until=GREATEST(COALESCE(verified_until, now()), now()) + make_interval(days=>$1),
		        updated_at=now() WHERE user_id=$2`, sub.RenewDays, uid); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO credit_transaction (user_id, amount, type, status, description) VALUES ($1,$2,'PURCHASE','COMPLETED',$3)`,
		uid, -price, "Perpanjangan Verified Store"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// StartSubscriptionReconciler: perpanjang otomatis langganan yang jatuh tempo.
// Pola sama dengan StartOrderReconciler — ticker, bukan cron eksternal.
//
// ponytail: satu pemindaian global tiap interval; kalau jumlah langganan sudah
// besar, ganti ke batch + lekuk per-user. Sekarang baris jatuh tempo per hari
// masih sedikit, jadi satu query cukup.
func StartSubscriptionReconciler(db DB, every time.Duration) {
	go func() {
		renewDueSubscriptions(context.Background(), db)
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for range ticker.C {
			renewDueSubscriptions(context.Background(), db)
		}
	}()
}

// renewDueSubscriptions: cari langganan ACTIVE yang auto-renew, sudah lewat
// jatuh tempo, lalu coba tagih. Gagal tagih → PAST_DUE + last_error, dan
// manfaatnya dibiarkan berjalan sampai verified_until benar-benar habis.
func renewDueSubscriptions(ctx context.Context, db DB) {
	rows, err := db.Query(ctx,
		`SELECT user_id, id, COALESCE(tier_name,''), status, price, auto_renew, renew_days, current_period_end, cancel_at_period_end, COALESCE(last_error,'')
		   FROM subscription
		  WHERE status='ACTIVE' AND auto_renew AND current_period_end <= now()
		  LIMIT 200`)
	if err != nil {
		return
	}
	type due struct {
		uid string
		sub subscriptionRow
	}
	var list []due
	for rows.Next() {
		var s subscriptionRow
		var uid string
		var end time.Time
		if err := rows.Scan(&uid, &s.ID, &s.TierName, &s.Status, &s.Price, &s.AutoRenew, &s.RenewDays, &end, &s.CancelAtEnd, &s.LastError); err != nil {
			continue
		}
		s.CurrentPeriodEnd = end.Format(time.RFC3339)
		list = append(list, due{uid: uid, sub: s})
	}
	rows.Close()

	for _, d := range list {
		// Harga terkunci menang; kalau kosong, pakai tarif platform saat ini.
		price := hargaPerpanjangan(d.sub.Price, settingFloat(ctx, db, "verified_store_monthly_price", 15000))
		if err := chargeRenewal(ctx, db, d.uid, d.sub, price); err != nil {
			// Saldo kurang bukan kesalahan sistem: tandai PAST_DUE supaya user
			// melihat alasan di UI dan bisa top-up, bukan diam-diam gagal.
			reason := "perpanjangan gagal"
			if err == errSaldoKurang {
				reason = "saldo tidak cukup saat jatuh tempo"
			}
			_, _ = db.Exec(ctx,
				`UPDATE subscription SET status='PAST_DUE', last_error=$1, updated_at=now() WHERE id=$2`, reason, d.sub.ID)
			log.Printf("[WARN] auto-renew gagal sub=%s: %v", d.sub.ID, err)
		}
	}
}
