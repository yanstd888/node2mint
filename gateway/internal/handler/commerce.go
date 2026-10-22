package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-sell/gateway/internal/auth"
	"ai-sell/gateway/internal/provider"

	"github.com/jackc/pgx/v5"
)

// Dompet, paket token, hadiah harian, referral, afiliasi, dan keranjang.
//
// Satu aturan yang dipegang di seluruh berkas ini: setiap perubahan saldo
// memakai syarat `wallet_balance >= jumlah` DI DALAM query dan meninggalkan
// satu baris credit_transaction. Cek-lalu-update di kode adalah cara paling
// umum saldo jadi minus.

func todayUTC() string { return time.Now().UTC().Format("2006-01-02") }

func randCode(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Grant menambah saldo user + satu baris buku besar dalam transaksi pemanggil.
func Grant(ctx context.Context, db DB, userID string, amount float64, kind, description string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE "user" SET wallet_balance = wallet_balance + $1 WHERE id=$2`, amount, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO credit_transaction (user_id, amount, type, status, description) VALUES ($1,$2,$3,'COMPLETED',$4)`,
		userID, amount, kind, description); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ------------------------------------------------------------------ DOMPET

// Wallet: GET /api/wallet → {balance, store_balance?}.
func Wallet(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var balance float64
		if err := db.QueryRow(r.Context(), `SELECT wallet_balance FROM "user" WHERE id=$1`, uid).Scan(&balance); err != nil {
			fail(w, http.StatusInternalServerError, "gagal memuat saldo")
			return
		}
		out := map[string]any{"balance": balance}
		// store_balance hanya bermakna untuk yang punya toko.
		var store float64
		if err := db.QueryRow(r.Context(), `SELECT wallet_balance FROM market_store WHERE user_id=$1`, uid).Scan(&store); err == nil {
			out["store_balance"] = store
		}
		ok(w, out)
	}
}

// Kesalahan transfer dompet toko yang perlu status HTTP berbeda.
var (
	errNoStore               = errors.New("toko tidak ditemukan")
	errInsufficientStoreFund = errors.New("saldo toko tidak mencukupi")
)

// transferStoreWallet memindahkan saldo dari dompet toko ke dompet user
// pemiliknya — supaya uang penjualan bisa ditarik lewat alur penarikan yang
// sudah ada. Escrow TIDAK dipisah di skema: hanya ada satu kolom saldo toko,
// jadi yang berpindah adalah saldo tersedia saat itu.
//
// Kepemilikan dan kecukupan saldo diperiksa DI DALAM satu UPDATE; tidak ada
// cek-lalu-update di kode yang bisa balapan.
func transferStoreWallet(ctx context.Context, db DB, uid string, amount float64, auditIP string) (string, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var storeID string
	err = tx.QueryRow(ctx,
		`UPDATE market_store SET wallet_balance = wallet_balance - $1, updated_at=now()
		  WHERE user_id=$2 AND wallet_balance >= $1 RETURNING id`, amount, uid).Scan(&storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Dua sebab mungkin: bukan pemilik toko, atau saldo kurang. Dibedakan
		// hanya supaya status HTTP benar; keduanya berarti "tidak jadi".
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM market_store WHERE user_id=$1)`, uid).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return "", errNoStore
		}
		return "", errInsufficientStoreFund
	}
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE "user" SET wallet_balance = wallet_balance + $1, updated_at=now() WHERE id=$2`, amount, uid); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO credit_transaction (user_id, amount, type, status, description, reference_id)
		 VALUES ($1,$2,'STORE_TRANSFER','COMPLETED','Pemindahan saldo toko ke dompet',$3)`,
		uid, amount, storeID); err != nil {
		return "", err
	}
	if err := auditTx(ctx, tx, uid, "STORE_WALLET_TRANSFER", storeID, nil, map[string]any{"amount": amount}, auditIP); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return storeID, nil
}

// TransferStoreWallet: POST /api/store/wallet/transfer {amount}.
func TransferStoreWallet(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			Amount num `json:"amount"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		// Bentuk !(a > 0), bukan a <= 0: NaN lolos perbandingan terakhir.
		amount := in.Amount.Float()
		if !(amount > 0) {
			fail(w, http.StatusBadRequest, "nominal tidak valid")
			return
		}
		_, err := transferStoreWallet(r.Context(), db, uid, amount, ipOf(r))
		switch {
		case errors.Is(err, errNoStore):
			fail(w, http.StatusNotFound, err.Error())
		case errors.Is(err, errInsufficientStoreFund):
			fail(w, http.StatusBadRequest, err.Error())
		case err != nil:
			log.Printf("[ERROR] transfer saldo toko gagal: user=%s err=%v", uid, err)
			fail(w, http.StatusInternalServerError, "gagal memindahkan saldo")
		default:
			ok(w, map[string]any{"ok": true, "amount": amount})
		}
	}
}

// DashboardUsage: riwayat pemakaian milik user sendiri. Hanya baris status 'OK'
// yang dihitung sebagai pemakaian sukses — baris gagal tetap terlihat, tapi
// dipisahkan supaya tidak terbaca sebagai panggilan berbayar yang berhasil.
func DashboardUsage(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		rows, err := db.Query(r.Context(),
			`SELECT model, input_tokens, output_tokens, cost, status, created_at
			   FROM usage_log WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type u struct {
			Model        string    `json:"model"`
			InputTokens  int       `json:"input_tokens"`
			OutputTokens int       `json:"output_tokens"`
			Cost         float64   `json:"cost"`
			Status       string    `json:"status"`
			CreatedAt    time.Time `json:"created_at"`
		}
		out := []u{}
		var totalCost float64
		for rows.Next() {
			var x u
			if err := rows.Scan(&x.Model, &x.InputTokens, &x.OutputTokens, &x.Cost, &x.Status, &x.CreatedAt); err != nil {
				log.Printf("[WARN] scan DashboardUsage: %v", err)
				continue
			}
			if x.Status == "OK" {
				totalCost += x.Cost
			}
			out = append(out, x)
		}
		ok(w, map[string]any{"usage": out, "total_cost": totalCost})
	}
}

// WalletTransactions: mutasi saldo (buku besar yang sama dengan saldo).
func WalletTransactions(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		limit, offset, page, per := pageOf(r)
		rows, err := db.Query(r.Context(),
			`SELECT id, amount, type, status, description, COALESCE(invoice_id,''), environment, created_at
			   FROM credit_transaction WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, uid, limit, offset)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type t struct {
			ID          string    `json:"id"`
			Amount      float64   `json:"amount"`
			Type        string    `json:"type"`
			Status      string    `json:"status"`
			Description string    `json:"description"`
			InvoiceID   string    `json:"invoice_id,omitempty"`
			Environment string    `json:"environment"`
			CreatedAt   time.Time `json:"created_at"`
		}
		out := []t{}
		for rows.Next() {
			var x t
			if err := rows.Scan(&x.ID, &x.Amount, &x.Type, &x.Status, &x.Description, &x.InvoiceID, &x.Environment, &x.CreatedAt); err != nil {
				continue
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in WalletTransactions: %v", err)
		}
		more := len(out) > per
		if more {
			out = out[:len(out)-1]
		}
		ok(w, map[string]any{"transactions": out, "page": page, "has_more": more})
	}
}

// PurchaseHistory: riwayat pembelian lintas produk marketplace dan paket token.
// Pesanan marketplace tetap memakai endpoint sendiri karena punya chat/escrow;
// halaman pembeli memakai agregat ini agar paket tidak hilang dari riwayat.
func PurchaseHistory(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		limit, offset, page, per := pageOf(r)
		rows, err := db.Query(r.Context(), `
			WITH purchases AS (
				SELECT c.id AS id, 'PACKAGE'::text AS kind, c.description AS product_title, 1 AS quantity, -c.amount AS subtotal, c.status, c.created_at, true AS is_instant, ''::text AS store_name
				  FROM credit_transaction c
				  JOIN user_token_package utp ON utp.id::text=c.invoice_id
				 WHERE c.user_id=$1 AND c.type='PURCHASE' AND c.status='COMPLETED'
				UNION ALL
				SELECT c.id, 'SERVICE'::text, c.description, 1, -c.amount, c.status, c.created_at, true, ''::text
				  FROM credit_transaction c
				 WHERE c.user_id=$1 AND c.type IN ('VERIFY_STORE','BOOST_PURCHASE') AND c.status='COMPLETED'
				UNION ALL
				SELECT o.id, 'MARKET'::text, p.title, o.qty, o.subtotal, o.status, o.created_at, p.is_instant, COALESCE(s.name,'')
				  FROM market_order o
				  JOIN market_product p ON p.id=o.product_id
				  LEFT JOIN market_store s ON s.id=o.store_id
				 WHERE o.buyer_id=$1
			)
			SELECT id, kind, product_title, quantity, subtotal, status, created_at, is_instant, store_name
			  FROM purchases ORDER BY created_at DESC LIMIT $2 OFFSET $3`, uid, limit, offset)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type purchase struct {
			ID           string    `json:"id"`
			Kind         string    `json:"kind"`
			ProductTitle string    `json:"product_title"`
			Quantity     int       `json:"quantity"`
			Subtotal     float64   `json:"subtotal"`
			Status       string    `json:"status"`
			CreatedAt    time.Time `json:"created_at"`
			IsInstant    bool      `json:"is_instant"`
			StoreName    string    `json:"store_name"`
		}
		out := []purchase{}
		for rows.Next() {
			var x purchase
			if err := rows.Scan(&x.ID, &x.Kind, &x.ProductTitle, &x.Quantity, &x.Subtotal, &x.Status, &x.CreatedAt, &x.IsInstant, &x.StoreName); err != nil {
				log.Printf("[WARN] scan PurchaseHistory: %v", err)
				continue
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in PurchaseHistory: %v", err)
		}
		more := len(out) > per
		if more {
			out = out[:len(out)-1]
		}
		ok(w, map[string]any{"orders": out, "page": page, "has_more": more})
	}
}

// PaymentMethods: detail tujuan pembayaran manual yang aman ditampilkan ke user.
func PaymentMethods(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keys := []string{"topup_bank_name", "topup_bank_account", "topup_bank_holder", "topup_qris_image_url", "topup_qris_merchant"}
		out := map[string]string{}
		for _, key := range keys {
			out[key] = settingStr(r.Context(), db, key, "")
		}
		ok(w, map[string]any{"methods": out})
	}
}

// Topup: membuat baris PENDING. Saldo TIDAK bertambah di sini — baru setelah
// admin memverifikasi transfer (nginx/route admin). Ini yang membedakan
// top-up nyata dari tombol yang mencetak uang.
func Topup(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			Amount    num    `json:"amount"`
			InvoiceID string `json:"invoice_id,omitempty"`
			Method    string `json:"method,omitempty"`
			ProofURL  string `json:"proof_url,omitempty"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		amount := in.Amount.Float()
		method := strings.ToLower(strings.TrimSpace(in.Method))
		reference := strings.TrimSpace(in.InvoiceID)
		proofURL := strings.TrimSpace(in.ProofURL)
		// Bentuk !(a && b), bukan a < x || a > y: tahan-NaN.
		if !(amount >= 10000 && amount <= 50000000) {
			fail(w, http.StatusBadRequest, "nominal tidak valid (Rp10.000 – Rp50.000.000)")
			return
		}
		if _, allowed := map[string]bool{"transfer": true, "qris": true}[method]; !allowed {
			fail(w, http.StatusBadRequest, "metode pembayaran tidak valid")
			return
		}
		proofID, hasProofPrefix := strings.CutPrefix(proofURL, "/api/media/")
		if !hasProofPrefix || proofID == "" {
			fail(w, http.StatusBadRequest, "bukti transfer wajib diunggah")
			return
		}
		var ownsProof bool
		if err := db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM media_asset WHERE id::text=$1 AND user_id=$2)`, proofID, uid).Scan(&ownsProof); err != nil || !ownsProof {
			fail(w, http.StatusBadRequest, "bukti transfer tidak valid")
			return
		}
		if reference == "" {
			reference = "TOPUP-" + strings.ToUpper(randCode(6))
		}
		if len(reference) < 3 || len(reference) > 100 {
			fail(w, http.StatusBadRequest, "referensi pembayaran wajib diisi")
			return
		}
		// Metode bayar yang dipilih user disimpan (reference_id): admin yang
		// memverifikasi transfer perlu tahu harus memeriksa ke mana.
		var id string
		err := db.QueryRow(r.Context(),
			`INSERT INTO credit_transaction (user_id, amount, type, status, description, invoice_id, reference_id, proof_url)
			 VALUES ($1,$2,'TOPUP','PENDING',$3,$4,$5,$6)
			 ON CONFLICT (user_id, invoice_id) WHERE type='TOPUP' DO NOTHING RETURNING id`,
			uid, amount, "Top-up manual "+method, reference, method, proofURL).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			var existingAmount float64
			var existingMethod string
			err = db.QueryRow(r.Context(),
				`SELECT id, amount, COALESCE(reference_id,'') FROM credit_transaction
				  WHERE user_id=$1 AND invoice_id=$2 AND type='TOPUP'`, uid, reference).
				Scan(&id, &existingAmount, &existingMethod)
			if err == nil && (existingAmount != amount || existingMethod != method) {
				fail(w, http.StatusConflict, "referensi pembayaran sudah dipakai untuk top-up lain")
				return
			}
		}
		if err != nil {
			log.Printf("[ERROR] topup insert gagal: %v", err)
			fail(w, http.StatusInternalServerError, "gagal membuat top-up")
			return
		}
		audit(r.Context(), db, uid, "TOPUP_REQUEST", id, nil, map[string]any{"amount": amount}, ipOf(r))
		ok(w, map[string]any{"id": id, "status": "PENDING", "amount": amount})
	}
}

// Withdrawals: GET /api/wallet/withdrawals.
func Withdrawals(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		rows, err := db.Query(r.Context(),
			`SELECT id, amount, status, COALESCE(payment_type,''), created_at, updated_at
			   FROM withdrawal WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type wd struct {
			ID          string    `json:"id"`
			Amount      float64   `json:"amount"`
			Status      string    `json:"status"`
			PaymentType string    `json:"payment_type,omitempty"`
			CreatedAt   time.Time `json:"created_at"`
			UpdatedAt   time.Time `json:"updated_at"`
		}
		out := []wd{}
		for rows.Next() {
			var x wd
			if err := rows.Scan(&x.ID, &x.Amount, &x.Status, &x.PaymentType, &x.CreatedAt, &x.UpdatedAt); err != nil {
				continue
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in Withdrawals: %v", err)
		}
		ok(w, map[string]any{"withdrawals": out})
	}
}

func withdrawalAmounts(amount, percent float64) (fee, net float64) {
	fee = math.Round(amount*percent) / 100
	return fee, amount - fee
}

// CreateWithdrawal: saldo DIPOTONG saat pengajuan (direservasi), bukan saat
// disetujui — kalau tidak, user bisa mengajukan berkali-kali dengan saldo sama.
func CreateWithdrawal(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			Amount          num    `json:"amount"`
			PayoutAccountID string `json:"payout_account_id"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		amount := in.Amount.Float()
		if !(amount > 0) {
			fail(w, http.StatusBadRequest, "nominal tidak valid")
			return
		}
		min := settingFloat(r.Context(), db, "withdrawal_min", 50000)
		if amount < min {
			fail(w, http.StatusBadRequest, "minimum penarikan Rp"+trimZero(min))
			return
		}
		var first, last, address, paymentType, providerName, identifier, accountName, kycStatus string
		if err := db.QueryRow(r.Context(),
			`SELECT COALESCE(u.first_name,''),COALESCE(u.last_name,''),COALESCE(u.address,''),p.type,p.provider,p.account_identifier,p.account_name,u.kyc_status
			   FROM "user" u JOIN payout_account p ON p.user_id=u.id WHERE u.id=$1 AND p.id=$2`, uid, strings.TrimSpace(in.PayoutAccountID)).
			Scan(&first, &last, &address, &paymentType, &providerName, &identifier, &accountName, &kycStatus); err != nil {
			fail(w, http.StatusBadRequest, "lengkapi profil dan pilih rekening penarikan yang valid")
			return
		}
		if kycStatus != "verified" {
			fail(w, http.StatusForbidden, "verifikasi identitas wajib sebelum penarikan")
			return
		}
		fullName := strings.Join(strings.Fields(first+" "+last), " ")
		if len(strings.TrimSpace(address)) < 10 || fullName == "" || !strings.EqualFold(fullName, strings.Join(strings.Fields(accountName), " ")) {
			fail(w, http.StatusBadRequest, "profil belum lengkap atau nama rekening tidak sama")
			return
		}
		feePercent := settingFloat(r.Context(), db, "withdrawal_fee_percent", 5)
		if !(feePercent >= 0 && feePercent <= 100) {
			feePercent = 5
		}
		fee, net := withdrawalAmounts(amount, feePercent)
		destination := providerName + " · " + identifier + " · a.n. " + accountName

		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		defer tx.Rollback(r.Context())

		cut, err := tx.Exec(r.Context(),
			`UPDATE "user" SET wallet_balance = wallet_balance - $1 WHERE id=$2 AND wallet_balance >= $1`, amount, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		if cut.RowsAffected() == 0 {
			fail(w, http.StatusBadRequest, "saldo tidak mencukupi")
			return
		}
		var id string
		if err := tx.QueryRow(r.Context(),
			`INSERT INTO withdrawal (user_id, amount, status, payment_type, destination, fee_amount, net_amount) VALUES ($1,$2,'PENDING',$3,$4,$5,$6) RETURNING id`,
			uid, amount, paymentType, destination, fee, net).Scan(&id); err != nil {
			fail(w, http.StatusInternalServerError, "gagal membuat penarikan")
			return
		}
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO credit_transaction (user_id, amount, type, status, description, invoice_id)
			 VALUES ($1,$2,'WITHDRAWAL','PENDING','Pengajuan penarikan saldo dompet',$3)`,
			uid, -amount, id); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mencatat mutasi")
			return
		}
		if err := auditTx(r.Context(), tx, uid, "WITHDRAWAL_REQUEST", id, nil, map[string]any{"amount": amount}, ipOf(r)); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mencatat audit")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan penarikan")
			return
		}
		ok(w, map[string]any{"ok": true, "id": id, "status": "PENDING", "fee": fee, "net": net})
	}
}

// ------------------------------------------------------------------ PAKET

// Packages: GET /api/packages (publik).
func Packages(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(),
			`SELECT id, name, token_count, bonus_tokens, price, duration_days, kind, list_price, promo_price, effective_days,
			        (price = 0 AND token_count = 2000000 AND lower(name) = 'free - just signup') AS is_signup
			 FROM token_package
			 WHERE is_active AND (price > 0 OR (token_count = 2000000 AND lower(name) = 'free - just signup'))
			 ORDER BY price, id`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type p struct {
			ID            string   `json:"id"`
			Name          string   `json:"name"`
			Tokens        int      `json:"tokens"`
			BonusTokens   int      `json:"bonus_tokens"`
			Price         float64  `json:"price"`
			Days          int      `json:"days"`
			Kind          string   `json:"kind"`
			ListPrice     *float64 `json:"list_price,omitempty"`
			PromoPrice    *float64 `json:"promo_price,omitempty"`
			EffectiveDays *int     `json:"effective_days,omitempty"`
			IsSignup      bool     `json:"is_signup,omitempty"`
		}
		out := []p{}
		for rows.Next() {
			var x p
			if err := rows.Scan(&x.ID, &x.Name, &x.Tokens, &x.BonusTokens, &x.Price, &x.Days,
				&x.Kind, &x.ListPrice, &x.PromoPrice, &x.EffectiveDays, &x.IsSignup); err != nil {
				continue
			}
			if x.IsSignup {
				x.Name = "Starter"
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in Packages: %v", err)
		}
		ok(w, map[string]any{"packages": out})
	}
}

// applyInactiveTokenPenalty menagih denda secara lazily ketika akun diaktifkan
// lagi. Tidak perlu scheduler; jumlah hari tertunda tetap dihitung tepat dan
// pengurangan dibatasi oleh saldo token sehingga tidak pernah negatif.
func applyInactiveTokenPenalty(ctx context.Context, tx pgx.Tx, userID string, overdueSeconds int64) error {
	const dailyPenalty = int64(25000)
	penalty := overdueSeconds / 86400 * dailyPenalty
	if penalty <= 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT id, remaining FROM user_token_package WHERE user_id=$1 AND NOT requires_telegram AND remaining>0 ORDER BY expires_at, id FOR UPDATE`, userID)
	if err != nil {
		return err
	}
	type balance struct {
		id        string
		remaining int64
	}
	var balances []balance
	for rows.Next() {
		var b balance
		if err := rows.Scan(&b.id, &b.remaining); err != nil {
			rows.Close()
			return err
		}
		balances = append(balances, b)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, b := range balances {
		if penalty <= 0 {
			break
		}
		taken := inactivePenalty(b.remaining, 1, penalty)
		if _, err := tx.Exec(ctx, `UPDATE user_token_package SET remaining=remaining-$1 WHERE id=$2`, taken, b.id); err != nil {
			return err
		}
		penalty -= taken
	}
	return nil
}

// BuyPackage: bayar paket dari saldo dompet. Uang keluar, kuota masuk — satu
// transaksi, jadi tidak ada paket yang didapat tanpa pembayaran.
//
// Surcharge: 25% (configurable via platform_setting `usage_surcharge_rate`)
// diterapkan PADA token usage SEBELUM multiplier di ChargeTx, bukan di sini.
// Di sini kita hanya mencatat purchased_price & effective_expires_at.
//
// Duration semantics:
//   - Jika user punya paket token aktif (expires_at > now, remaining > 0),
//     paket baru EXTEND (perpanjang expires_at) dan tambah remaining.
//   - Jika tidak, OVERWRITE — insert baris baru.
func BuyPackage(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")
		requestKey, valid := idempotencyKey(w, r)
		if !valid {
			return
		}

		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		defer tx.Rollback(r.Context())
		cached, err := claimIdempotency(r.Context(), tx, uid, "package_buy", requestKey, id)
		if err != nil {
			failIdempotency(w, err)
			return
		}
		if cached != nil {
			replayIdempotency(w, cached)
			return
		}

		var name string
		var tokens, days int
		var price float64
		var effDaysAny any
		if err := tx.QueryRow(r.Context(),
			`SELECT name, token_count, price, duration_days, effective_days FROM token_package WHERE id=$1 AND is_active`, id).
			Scan(&name, &tokens, &price, &days, &effDaysAny); err != nil {
			fail(w, http.StatusNotFound, "paket tidak ditemukan")
			return
		}
		if price <= 0 || strings.EqualFold(strings.TrimSpace(name), "Free - Just Signup") {
			fail(w, http.StatusConflict, "paket Starter hanya diberikan sekali saat signup")
			return
		}

		// Jika paket punya effective_days override, pakai itu; else duration_days.
		effDays := days
		if effDaysAny != nil {
			if ed, ok := effDaysAny.(int); ok && ed > 0 {
				effDays = ed
			}
		}

		cut, err := tx.Exec(r.Context(),
			`UPDATE "user" SET wallet_balance = wallet_balance - $1 WHERE id=$2 AND wallet_balance >= $1`, price, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses pembayaran")
			return
		}
		if cut.RowsAffected() == 0 {
			fail(w, http.StatusPaymentRequired, "saldo tidak cukup")
			return
		}

		// Ambil masa aktif terakhir, termasuk paket kedaluwarsa yang masih punya
		// token: token tidak pernah dibuang; pembelian berikutnya mengaktifkannya lagi.
		var activeUTPID string
		var remaining float64
		var currentExpiry int64
		err = tx.QueryRow(r.Context(),
			`SELECT id, remaining, EXTRACT(EPOCH FROM expires_at)::bigint FROM user_token_package
			 WHERE user_id=$1 AND NOT requires_telegram AND remaining > 0
			 ORDER BY expires_at DESC LIMIT 1 FOR UPDATE`, uid).
			Scan(&activeUTPID, &remaining, &currentExpiry)
		hasActive := err == nil
		nowUnix := time.Now().Unix()

		if hasActive {
			if currentExpiry < nowUnix {
				if err := applyInactiveTokenPenalty(r.Context(), tx, uid, nowUnix-currentExpiry); err != nil {
					fail(w, http.StatusInternalServerError, "gagal menerapkan denda masa tidak aktif")
					return
				}
			}
			// EXTEND dari expiry terakhir (bukan dari now), lalu aktifkan kembali
			// seluruh sisa token agar token expired tidak hilang.
			newExp := accumulatedExpiry(nowUnix, currentExpiry, effDays)
			_, err = tx.Exec(r.Context(),
				`UPDATE user_token_package
				 SET expires_at = to_timestamp($1), effective_expires_at = to_timestamp($1)
				 WHERE user_id=$2 AND NOT requires_telegram`, newExp, uid)
			if err == nil {
				_, err = tx.Exec(r.Context(), `UPDATE user_token_package SET remaining=remaining+$1 WHERE id=$2`, tokens, activeUTPID)
			}
			if err != nil {
				fail(w, http.StatusInternalServerError, "gagal memperpanjang paket")
				return
			}
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO credit_transaction (user_id, amount, type, status, description, invoice_id)
				 VALUES ($1,$2,'PURCHASE','COMPLETED',$3,$4)`,
				uid, -price, "Beli paket "+name, activeUTPID); err != nil {
				fail(w, http.StatusInternalServerError, "gagal mencatat mutasi")
				return
			}
			if err := payReferralTokenCommission(r.Context(), tx, uid, activeUTPID, tokens, effDays); err != nil {
				fail(w, http.StatusInternalServerError, "gagal membayar reward referral")
				return
			}
			out := map[string]any{"ok": true, "id": activeUTPID, "tokens": tokens,
				"expires_days": effDays, "action": "extend"}
			if err := completeIdempotency(r.Context(), tx, uid, "package_buy", requestKey, out); err != nil {
				fail(w, http.StatusInternalServerError, "gagal mencatat idempotency request")
				return
			}
			if err := tx.Commit(r.Context()); err != nil {
				fail(w, http.StatusInternalServerError, "gagal menyimpan pembelian")
				return
			}
			audit(r.Context(), db, uid, "PACKAGE_BUY", activeUTPID, nil, map[string]any{"package": name, "price": price, "tokens": tokens, "action": "extend"}, ipOf(r))
			ok(w, out)
		} else {
			// OVERWRITE: insert baru.
			var utpID string
			if err := tx.QueryRow(r.Context(),
				`INSERT INTO user_token_package
				 (user_id, token_package_id, expires_at, remaining, purchased_price, effective_expires_at, grant_source, requires_telegram)
				 VALUES ($1,$2, to_timestamp($3), $4, $5, to_timestamp($3),'paid',false)
				 RETURNING id`,
				uid, id, extendExpiry(time.Now().Unix(), effDays), tokens, price).
				Scan(&utpID); err != nil {
				log.Printf("[ERROR] simpan paket gagal: user=%s pkg=%s err=%v", uid, id, err)
				fail(w, http.StatusInternalServerError, "gagal menyimpan paket")
				return
			}
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO credit_transaction (user_id, amount, type, status, description, invoice_id)
				 VALUES ($1,$2,'PURCHASE','COMPLETED',$3,$4)`,
				uid, -price, "Beli paket "+name, utpID); err != nil {
				fail(w, http.StatusInternalServerError, "gagal mencatat mutasi")
				return
			}
			if err := payReferralTokenCommission(r.Context(), tx, uid, utpID, tokens, effDays); err != nil {
				fail(w, http.StatusInternalServerError, "gagal membayar reward referral")
				return
			}
			out := map[string]any{"ok": true, "id": utpID, "tokens": tokens,
				"expires_days": effDays, "action": "overwrite"}
			if err := completeIdempotency(r.Context(), tx, uid, "package_buy", requestKey, out); err != nil {
				fail(w, http.StatusInternalServerError, "gagal mencatat idempotency request")
				return
			}
			if err := tx.Commit(r.Context()); err != nil {
				fail(w, http.StatusInternalServerError, "gagal menyimpan pembelian")
				return
			}
			audit(r.Context(), db, uid, "PACKAGE_BUY", utpID, nil, map[string]any{"package": name, "price": price, "tokens": tokens, "action": "overwrite"}, ipOf(r))
			ok(w, out)
		}
	}
}

// ------------------------------------------------------------------ REWARD

// DailyRewardStatus: GET /api/reward/daily — status klaim hari ini + runtutan +
// riwayat 30 klaim terakhir (dipakai halaman /reward).
func DailyRewardStatus(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		day := todayUTC()
		var amount float64
		claimed := false
		if err := db.QueryRow(r.Context(),
			`SELECT amount FROM daily_reward_claim WHERE user_id=$1 AND reward_date=$2`, uid, day).Scan(&amount); err == nil {
			claimed = true
		}
		rows, err := db.Query(r.Context(),
			`SELECT reward_date, amount FROM daily_reward_claim WHERE user_id=$1 ORDER BY reward_date DESC LIMIT 30`, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type claim struct {
			Date   string  `json:"reward_date"`
			Amount float64 `json:"amount"`
		}
		var hist []claim
		streak := 0
		for rows.Next() {
			var x claim
			if err := rows.Scan(&x.Date, &x.Amount); err != nil {
				continue
			}
			hist = append(hist, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in DailyRewardStatus: %v", err)
		}
		// Runtutan: klaim hari berturut-turut yang berakhir di hari ini, atau di
		// kemarin kalau hari ini belum diklaim (belum klaim ≠ runtutan putus).
		want := day
		if !claimed {
			t, err := time.Parse("2006-01-02", day)
			if err == nil {
				want = t.AddDate(0, 0, -1).Format("2006-01-02")
			}
		}
		for _, x := range hist {
			if x.Date != want {
				break
			}
			streak++
			t, err := time.Parse("2006-01-02", want)
			if err != nil {
				break
			}
			want = t.AddDate(0, 0, -1).Format("2006-01-02")
		}
		out := map[string]any{
			"date": day, "claimed": claimed, "amount": amount, "streak": streak, "history": hist,
			"reward": settingFloat(r.Context(), db, "daily_reward_amount", 100),
		}
		// Nama yang dipakai halaman frontend (claimedToday) — alias dari claimed.
		out["claimed_today"] = claimed
		ok(w, out)
	}
}

// ClaimDailyReward: klaim ditulis DI DALAM transaksi bersama penambahan saldo
// dan unique(user_id, reward_date) di DB yang menentukan pemenangnya — bukan
// cek-lalu-insert di kode (dua klik bersamaan bisa membayar dua kali).
func ClaimDailyReward(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		if !RequireTelegramFeatureHTTP(w, r, db, telegramFeatureDaily) {
			return
		}
		day := todayUTC()
		amount := settingFloat(r.Context(), db, "daily_reward_amount", 100)
		ip := ipOf(r)
		if ip != "" {
			var distinct int
			_ = db.QueryRow(r.Context(), `SELECT COUNT(DISTINCT user_id) FROM audit_log WHERE action='CHECKIN_CLAIM' AND ip=$1 AND created_at > now()-interval '24 hours'`, ip).Scan(&distinct)
			if distinct >= int(settingFloat(r.Context(), db, "daily_reward_ip_limit", 3)) {
				_, _ = db.Exec(r.Context(), `UPDATE "user" SET fraud_flag=true, fraud_reason='daily check-in IP velocity' WHERE id=$1`, uid)
				audit(r.Context(), db, uid, "CHECKIN_HOLD", uid, nil, map[string]any{"ip": ip, "distinct_users_24h": distinct}, ip)
				fail(w, http.StatusTooManyRequests, "klaim ditahan untuk review keamanan")
				return
			}
		}

		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		defer tx.Rollback(r.Context())

		ins, err := tx.Exec(r.Context(),
			`INSERT INTO daily_reward_claim (user_id, reward_date, amount) VALUES ($1,$2,$3)
			 ON CONFLICT (user_id, reward_date) DO NOTHING`, uid, day, amount)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memproses")
			return
		}
		if ins.RowsAffected() == 0 {
			fail(w, http.StatusBadRequest, "hadiah hari ini sudah diambil")
			return
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE "user" SET wallet_balance = wallet_balance + $1 WHERE id=$2`, amount, uid); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menambah saldo")
			return
		}
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO credit_transaction (user_id, amount, type, status, description)
			 VALUES ($1,$2,'REWARD','COMPLETED',$3)`, uid, amount, "Hadiah harian "+day); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mencatat hadiah")
			return
		}
		if err := auditTx(r.Context(), tx, uid, "CHECKIN_CLAIM", uid, nil, map[string]any{"ip": ipOf(r), "reward": amount, "date": day}, ipOf(r)); err != nil {
			fail(w, http.StatusInternalServerError, "gagal mencatat audit")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan klaim")
			return
		}
		ok(w, map[string]any{"ok": true, "amount": amount, "date": day})
	}
}

// ------------------------------------------------------------------ REFERRAL

// Referral: GET /api/referral — kode sendiri + statistik.
// Reward referral = PAKET TOKEN API (bukan wallet/uang). API mengembalikan
// token count, bukan nilai rupiah. UI menampilkan token.
func Referral(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		code := ensureReferralCode(r.Context(), db, uid)

		var signups int
		_ = db.QueryRow(r.Context(), `SELECT COUNT(*) FROM referral_signup WHERE inviter_id=$1`, uid).Scan(&signups)
		weeklyCount, weeklyLimit, resetAt := referralWeeklyStatus(r.Context(), db, uid, time.Now().UTC())

		// Total token reward yang sudah diberikan (dari paket token 'Reward Referral').
		var totalTokens float64
		_ = db.QueryRow(r.Context(),
			`SELECT COALESCE(SUM(remaining),0) FROM user_token_package utp
			   JOIN token_package tp ON tp.id = utp.token_package_id
			  WHERE tp.name='Reward Referral' AND utp.user_id=$1`, uid).Scan(&totalTokens)

		// Reward signup selalu mengikuti setting global (2.5M default), bukan
		// nilai lama 5.000 yang mungkin masih tersimpan di referral_code.
		var reward float64 = defaultReferralReward
		var sr string
		if err := db.QueryRow(r.Context(), `SELECT value FROM platform_setting WHERE key='referral_reward_tokens'`).Scan(&sr); err == nil {
			if f, err := strconv.ParseFloat(sr, 64); err == nil && f > 0 {
				reward = f
			}
		}
		var maxUses *int
		var used int
		_ = db.QueryRow(r.Context(), `SELECT max_uses, used_count FROM referral_code WHERE code=$1`, code).Scan(&maxUses, &used)

		// Riwayat signup: invitee email + apakah bonus sudah dibayar (token diberikan).
		rows, err := db.Query(r.Context(),
			`SELECT s.id, COALESCE(u.email,''), s.bonus_given, s.created_at
			   FROM referral_signup s LEFT JOIN "user" u ON u.id = s.invitee_id
			  WHERE s.inviter_id=$1 ORDER BY s.created_at DESC LIMIT 50`, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type signupRow struct {
			ID        string    `json:"id"`
			Invitee   string    `json:"invitee"`
			Paid      bool      `json:"paid"`
			CreatedAt time.Time `json:"created_at"`
		}
		var signupsList []signupRow
		for rows.Next() {
			var x signupRow
			if err := rows.Scan(&x.ID, &x.Invitee, &x.Paid, &x.CreatedAt); err != nil {
				continue
			}
			signupsList = append(signupsList, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in Referral: %v", err)
		}

		ok(w, map[string]any{
			"code": code, "myCode": code, "reward": reward,
			"max_uses": maxUses, "used_count": used,
			"total_tokens":      int(totalTokens),
			"reward_per_signup": int(reward),
			"total_referrals":   signups,
			"weekly_count":      weeklyCount, "weekly_limit": weeklyLimit, "weekly_reset_at": resetAt,
			"summary": map[string]any{
				"signups":     signups,
				"totalInvite": signups,
				"totalTokens": int(totalTokens),
			},
			"signups": signupsList,
		})
	}
}

const referralWeeklyLimit = 15

func referralWeeklyWindow(first, now time.Time) (time.Time, time.Time) {
	if now.Before(first) {
		now = first
	}
	period := int(now.Sub(first) / (7 * 24 * time.Hour))
	start := first.Add(time.Duration(period) * 7 * 24 * time.Hour)
	return start, start.Add(7 * 24 * time.Hour)
}

func referralWeeklyStatus(ctx context.Context, db DB, inviter string, now time.Time) (int, int, *time.Time) {
	var first *time.Time
	if err := db.QueryRow(ctx, `SELECT MIN(created_at) FROM referral_signup WHERE inviter_id=$1`, inviter).Scan(&first); err != nil || first == nil {
		return 0, referralWeeklyLimit, nil
	}
	start, reset := referralWeeklyWindow(*first, now)
	var count int
	_ = db.QueryRow(ctx, `SELECT COUNT(*) FROM referral_signup WHERE inviter_id=$1 AND created_at >= $2 AND created_at < $3`, inviter, start, reset).Scan(&count)
	return count, referralWeeklyLimit, &reset
}

// ensureReferralCode membuat kode undangan user kalau belum ada. Kode disimpan
// di dua tempat (user.referral_code dan referral_code) supaya link undangan
// bisa punya reward/kuota tanpa kolom baru.
func ensureReferralCode(ctx context.Context, db DB, uid string) string {
	var code string
	if err := db.QueryRow(ctx, `SELECT COALESCE(referral_code,'') FROM "user" WHERE id=$1`, uid).Scan(&code); err == nil && code != "" {
		return code
	}
	code = "AI" + randCode(4)
	tx, err := db.Begin(ctx)
	if err != nil {
		return code
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE "user" SET referral_code=$1 WHERE id=$2 AND referral_code IS NULL`, code, uid); err != nil {
		return code
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO referral_code (code, created_by, type, reward) VALUES ($1,$2,'SIGNUP',1000000)
		 ON CONFLICT (code) DO NOTHING`, code, uid); err != nil {
		return code
	}
	if err := tx.Commit(ctx); err != nil {
		return code
	}
	return code
}

// grantSignupFreePackage grants the one-time signup entitlement; it is not a purchase.
func grantSignupFreePackage(ctx context.Context, db DB, userID string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Lock the account so a repeated registration hook cannot grant a second
	// lifetime Starter entitlement.
	var lockedUserID string
	if err := tx.QueryRow(ctx, `SELECT id FROM "user" WHERE id=$1 FOR UPDATE`, userID).Scan(&lockedUserID); err != nil {
		return err
	}
	var alreadyGranted bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM user_token_package utp
		JOIN token_package tp ON tp.id=utp.token_package_id
		WHERE utp.user_id=$1 AND tp.token_count=2000000
		  AND lower(tp.name)='free - just signup'
	)`, userID).Scan(&alreadyGranted); err != nil {
		return err
	}
	if alreadyGranted {
		return tx.Commit(ctx)
	}
	var pkgID string
	if err := tx.QueryRow(ctx, `SELECT id FROM token_package WHERE name='Free - Just Signup' AND token_count=2000000 AND is_active LIMIT 1`).Scan(&pkgID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_token_package (user_id, token_package_id, expires_at, remaining, purchased_price, effective_expires_at, grant_source, requires_telegram) VALUES ($1,$2,now()+interval '1 day',$3,0,now()+interval '1 day','signup',true)`, userID, pkgID, 2000000); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// extendActiveTokenExpiry menambah `days` hari HANYA ke masa aktif yang masih
// berlaku. Baris yang sudah kedaluwarsa tidak ikut diaktifkan — itu hanya lewat
// pembelian paket (dan kena denda masa tidak aktif).
func extendActiveTokenExpiry(ctx context.Context, tx pgx.Tx, userID string, days int) error {
	if days <= 0 {
		return nil
	}
	_, err := tx.Exec(ctx,
		`UPDATE user_token_package
		    SET expires_at = expires_at + make_interval(days=>$1),
		        effective_expires_at = expires_at + make_interval(days=>$1)
		  WHERE user_id=$2 AND expires_at > now()`, days, userID)
	return err
}

// grantReferralTokens menyimpan baris reward token yang MENEMPEL pada masa aktif
// yang sedang berjalan (tidak menambah durasi); kalau tidak ada masa aktif,
// baris baru memakai `freshDays` hari.
func grantReferralTokens(ctx context.Context, tx pgx.Tx, userID, pkgID string, tokens, freshDays int, source string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO user_token_package (user_id, token_package_id, expires_at, remaining, purchased_price, effective_expires_at, grant_source, requires_telegram)
		 SELECT $1, $2, e, $4, 0, e, $5, true FROM (
		   SELECT COALESCE(
		     (SELECT MAX(expires_at) FROM user_token_package WHERE user_id=$1 AND expires_at > now()),
		     now() + make_interval(days=>$3)) AS e
		 ) s`, userID, pkgID, freshDays, tokens, source)
	return err
}

// TOKEN API untuk kedua sisi (bukan wallet: wallet = uang real marketplace,
// token = kuota AI; keputusan user). Paket exp 30 hari, nominal = reward kode.
// Tanpa baris ledger wallet — paket token punya buku sendiri (remaining).
// Dalam transaksi pemanggil.
func referralBonus(ctx context.Context, tx pgx.Tx, invitee, inviter, code string, reward float64) error {
	tok := int(reward)
	if tok <= 0 {
		return nil
	}
	var pkgID string
	err := tx.QueryRow(ctx,
		`SELECT id FROM token_package WHERE name='Reward Referral' AND token_count=$1 AND NOT is_active LIMIT 1`, tok).
		Scan(&pkgID)
	if err != nil {
		if err := tx.QueryRow(ctx,
			`INSERT INTO token_package (name, token_count, price, duration_days, is_active)
			 VALUES ('Reward Referral', $1, 0, 30, false) RETURNING id`, tok).Scan(&pkgID); err != nil {
			return err
		}
	}
	// Referral = TOKEN saja. Masa aktif yang diberikan hanya 1 HARI (bukan 30),
	// dan bagi pemberi referral ditambahkan ke periode yang sedang berjalan.
	if err := extendActiveTokenExpiry(ctx, tx, inviter, 1); err != nil {
		return err
	}
	if err := grantReferralTokens(ctx, tx, inviter, pkgID, tok, 1, "referral"); err != nil {
		return err
	}
	return grantReferralTokens(ctx, tx, invitee, pkgID, tok, 1, "referral")
}

// payReferralTokenCommission gives inviter 10% of a purchased package's tokens,
// capped at two qualifying purchases per invitee. It never touches Wallet.
func payReferralTokenCommission(ctx context.Context, tx pgx.Tx, invitee, source string, tokens, days int) error {
	var inviter string
	var count int
	if err := tx.QueryRow(ctx, `SELECT inviter_id, qualifying_topups FROM referral_signup WHERE invitee_id=$1 FOR UPDATE`, invitee).Scan(&inviter, &count); err != nil || inviter == invitee || count >= 2 {
		return nil
	}
	var pctRaw string
	pct := 10.0
	if err := tx.QueryRow(ctx, `SELECT value FROM platform_setting WHERE key='referral_commission_percent'`).Scan(&pctRaw); err == nil {
		if parsed, err := strconv.ParseFloat(pctRaw, 64); err == nil {
			pct = parsed
		}
	}
	reward := int(math.Floor(float64(tokens) * pct / 100))
	if reward <= 0 {
		return nil
	}
	var claimed bool
	if err := tx.QueryRow(ctx, `INSERT INTO referral_commission (inviter_id, invitee_id, amount, source_txn_id, status) VALUES ($1,$2,$3,$4,'PAID') ON CONFLICT (source_txn_id) DO NOTHING RETURNING true`, inviter, invitee, reward, source).Scan(&claimed); err != nil || !claimed {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	var pkgID string
	if err := tx.QueryRow(ctx, `SELECT id FROM token_package WHERE name='Reward Referral' AND token_count=$1 AND NOT is_active LIMIT 1`, reward).Scan(&pkgID); err != nil {
		if err := tx.QueryRow(ctx, `INSERT INTO token_package (name, token_count, price, duration_days, is_active) VALUES ('Reward Referral',$1,0,$2,false) RETURNING id`, reward, days).Scan(&pkgID); err != nil {
			return err
		}
	}
	// Komisi referral = token saja; masa aktif tidak ditambah.
	if err := grantReferralTokens(ctx, tx, inviter, pkgID, reward, 1, "referral_commission"); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE referral_signup SET qualifying_topups=qualifying_topups+1 WHERE invitee_id=$1`, invitee)
	return err
}

// (?aff=…), bonus kedua sisi dibayar saat pendaftaran. unique(invitee_id)
// membuat bonus tidak bisa digandakan walau request daftar dikirim ulang.
//
// Anti-abuse: jika inviter dan invitee memakai IP yang sama saat daftar, bonus
// tidak diberikan; IP disimpan di-hash (provider.HashKey + auth.Secret).
func affiliateSignup(ctx context.Context, db DB, userID, code, ipHash string) (bool, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	tx, err := db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var inviter string
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(created_by::text,'') FROM referral_code
	  WHERE upper(code)=$1 AND (expires_at IS NULL OR expires_at > now())
	    AND (max_uses IS NULL OR used_count < max_uses) FOR UPDATE`, code).Scan(&inviter); err != nil || inviter == "" || inviter == userID {
		return false, nil
	}
	// Baris referral_code terkunci di atas: cap check + insert aman dari balapan.
	var first *time.Time
	if err := tx.QueryRow(ctx, `SELECT MIN(created_at) FROM referral_signup WHERE inviter_id=$1`, inviter).Scan(&first); err != nil {
		return true, err
	}
	if first != nil {
		start, reset := referralWeeklyWindow(*first, time.Now().UTC())
		var count int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM referral_signup WHERE inviter_id=$1 AND created_at >= $2 AND created_at < $3`, inviter, start, reset).Scan(&count); err != nil {
			return true, err
		}
		if count >= referralWeeklyLimit {
			return false, nil
		}
	}
	tag, err := tx.Exec(ctx,
		`INSERT INTO referral_signup (inviter_id, invitee_id, code, signup_ip_hash, bonus_given) VALUES ($1,$2,$3,$4,false)
		 ON CONFLICT (invitee_id) DO NOTHING`, inviter, userID, code, ipHash)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return true, nil
	}
	var inviterIP string
	var reusedIP bool
	if err := tx.QueryRow(ctx, `SELECT COALESCE(signup_ip_hash,'') FROM "user" WHERE id=$1`, inviter).Scan(&inviterIP); err != nil {
		return true, err
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM "user" WHERE signup_ip_hash=$1 AND id<>$2)`, ipHash, userID).Scan(&reusedIP); err != nil {
		return true, err
	}
	if referralEligible(inviterIP, ipHash, false, reusedIP) {
		if _, err := tx.Exec(ctx, `UPDATE referral_code SET used_count = used_count + 1 WHERE upper(code)=$1`, code); err != nil {
			return true, err
		}
		reward := float64(defaultReferralReward)
		var rawReward string
		if err := tx.QueryRow(ctx, `SELECT value FROM platform_setting WHERE key='referral_reward_tokens'`).Scan(&rawReward); err == nil {
			if parsed, err := strconv.ParseFloat(rawReward, 64); err == nil && parsed > 0 {
				reward = parsed
			}
		}
		if err := referralBonus(ctx, tx, userID, inviter, code, reward); err != nil {
			return true, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE referral_signup SET bonus_given=true WHERE invitee_id=$1`, userID); err != nil {
		return true, err
	}
	return true, tx.Commit(ctx)
}

// payDeferredReferralBonus: lunasi bonus undangan (kedua sisi) pada topup
// PERTAMA invitee. Dipanggil dari SettleTopup dan StripeWebhook di dalam
// transaksi penyelesaian topup; bonus_given mencegah pembayaran ulang.
//
// Anti-abuse: melewati pembayaran (tapi tetap tandai bonus_given=true) ketika
// inviter dan invitee mendaftar dari IP yang SAMA — berarti akun dummy/bot.
func payDeferredReferralBonus(ctx context.Context, tx pgx.Tx, invitee string) {
	var inviter, code, inviteeIP, inviterIP string
	var reusedIP bool
	if err := tx.QueryRow(ctx, `SELECT inviter_id::text, code, COALESCE(signup_ip_hash,'')
		FROM referral_signup
		WHERE invitee_id=$1 AND NOT bonus_given FOR UPDATE`, invitee).Scan(&inviter, &code, &inviteeIP); err != nil {
		return
	}
	// Cek IP inviter: lihat signup_ip_hash di tabel user (diisi di Register).
	if inviter != "" {
		_ = tx.QueryRow(ctx, `SELECT COALESCE(signup_ip_hash,'') FROM "user" WHERE id=$1`, inviter).Scan(&inviterIP)
	}
	if inviteeIP != "" {
		_ = tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM referral_signup WHERE signup_ip_hash=$1 AND invitee_id<>$2
		)`, inviteeIP, invitee).Scan(&reusedIP)
	}
	// Same-IP = dummy account farming: tandai lunas, jangan bayar.
	if referralEligible(inviterIP, inviteeIP, inviter == invitee, reusedIP) {
		var reward float64 = defaultReferralReward
		var sr string
		if err := tx.QueryRow(ctx, `SELECT value FROM platform_setting WHERE key='referral_reward_tokens'`).Scan(&sr); err == nil {
			if f, err := strconv.ParseFloat(sr, 64); err == nil && f > 0 {
				reward = f
			}
		}
		if reward > 0 {
			if err := referralBonus(ctx, tx, invitee, inviter, code, reward); err != nil {
				log.Printf("[WARN] bonus referral tertunda gagal: %v", err)
				return
			}
		}
	} else {
		log.Printf("[WARN] referral bonus dilewati: invitee=%s inviter=%s (IP sama/self)", invitee, inviter)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE referral_signup SET bonus_given=true WHERE invitee_id=$1`, invitee); err != nil {
		log.Printf("[WARN] tandai bonus referral lunas gagal: %v", err)
	}
}

// ------------------------------------------------------------------ AFILIASI

// AffiliateLinks: GET /api/affiliate/links — statistik per link milik sendiri.
func AffiliateLinks(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		rows, err := db.Query(r.Context(),
			`SELECT l.id, l.code, l.is_active, l.product_id, pr.title, pr.price, l.created_at,
			        (SELECT COUNT(*) FROM affiliate_click c WHERE c.link_id = l.id),
			        (SELECT COUNT(*) FROM affiliate_conversion v WHERE v.link_id = l.id),
			        (SELECT COALESCE(SUM(commission_amount),0) FROM affiliate_conversion v WHERE v.link_id = l.id)
			   FROM affiliate_link l JOIN market_product pr ON pr.id = l.product_id
			  WHERE l.user_id=$1 AND pr.allow_affiliate AND pr.affiliate_percent > 0
			  ORDER BY l.created_at DESC`, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type l struct {
			ID        string `json:"id"`
			Code      string `json:"code"`
			IsActive  bool   `json:"is_active"`
			ProductID string `json:"product_id"`
			Title     string `json:"product_title"`
			// Nama yang dibaca halaman /afiliasi (l.title, l.earnings).
			Title2      string    `json:"title,omitempty"`
			Price       float64   `json:"product_price"`
			CreatedAt   time.Time `json:"created_at"`
			Clicks      int       `json:"clicks"`
			Conversions int       `json:"conversions"`
			Revenue     float64   `json:"revenue"`
			Earnings    float64   `json:"earnings"`
		}
		out := []l{}
		var totalClicks, totalConv int
		var totalRevenue float64
		for rows.Next() {
			var x l
			if err := rows.Scan(&x.ID, &x.Code, &x.IsActive, &x.ProductID, &x.Title, &x.Price, &x.CreatedAt,
				&x.Clicks, &x.Conversions, &x.Revenue); err != nil {
				continue
			}
			x.Title2 = x.Title
			x.Earnings = x.Revenue
			out = append(out, x)
			totalClicks += x.Clicks
			totalConv += x.Conversions
			totalRevenue += x.Revenue
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in AffiliateLinks: %v", err)
		}
		ok(w, map[string]any{
			"links": out,
			"summary": map[string]any{
				"total_links": len(out), "total_clicks": totalClicks,
				"total_conversions": totalConv, "total_revenue": totalRevenue,
				// Nama yang dibaca halaman /afiliasi (ringkas.earnings).
				"earnings": totalRevenue, "clicks": totalClicks, "conversions": totalConv,
			},
		})
	}
}

// CreateAffiliateLink: POST /api/affiliate/links {product_id}.
func CreateAffiliateLink(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			ProductID string `json:"product_id"`
		}
		if err := decode(r, &in); err != nil || in.ProductID == "" {
			fail(w, http.StatusBadRequest, "product_id wajib")
			return
		}

		var allow bool
		var percent float64
		var owner string
		if err := db.QueryRow(r.Context(),
			`SELECT allow_affiliate, affiliate_percent, COALESCE(s.user_id::text,'')
			   FROM market_product p LEFT JOIN market_store s ON s.id = p.store_id
			  WHERE p.id=$1 AND p.is_active AND p.deleted_at IS NULL`,
			in.ProductID).Scan(&allow, &percent, &owner); err != nil {
			fail(w, http.StatusNotFound, "produk tidak ditemukan")
			return
		}
		if !allow || percent <= 0 {
			fail(w, http.StatusBadRequest, "produk ini tidak mengizinkan afiliasi")
			return
		}
		if owner == uid {
			fail(w, http.StatusBadRequest, "tidak bisa mengafiliasikan produk sendiri")
			return
		}

		// Link lama yang dimatikan dihidupkan lagi, bukan dibuat baru — supaya
		// statistik klik/konversi sebelumnya tidak terpisah.
		var code string
		var active bool
		err := db.QueryRow(r.Context(),
			`SELECT code, is_active FROM affiliate_link WHERE product_id=$1 AND user_id=$2`, in.ProductID, uid).Scan(&code, &active)
		if err == nil {
			if !active {
				if _, err := db.Exec(r.Context(),
					`UPDATE affiliate_link SET is_active=true WHERE product_id=$1 AND user_id=$2`, in.ProductID, uid); err != nil {
					fail(w, http.StatusInternalServerError, "gagal mengaktifkan link")
					return
				}
			}
		} else {
			code = randCode(5)
			if _, err := db.Exec(r.Context(),
				`INSERT INTO affiliate_link (product_id, user_id, code) VALUES ($1,$2,$3)`,
				in.ProductID, uid, code); err != nil {
				log.Printf("[ERROR] buat link afiliasi gagal: %v", err)
				fail(w, http.StatusInternalServerError, "gagal membuat link")
				return
			}
		}
		ok(w, map[string]any{"ok": true, "code": code, "url": "/market?aff=" + code})
	}
}

// AffiliateClick: publik (pengunjung yang mengklik belum tentu punya akun).
// IP di-hash dengan JWT_SECRET supaya tidak bisa dibalik jadi alamat asli.
func AffiliateClick(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		if code == "" {
			fail(w, http.StatusBadRequest, "kode wajib")
			return
		}
		var linkID string
		if err := db.QueryRow(r.Context(),
			`SELECT l.id FROM affiliate_link l
			   JOIN market_product p ON p.id=l.product_id
			  WHERE l.code=$1 AND l.is_active AND p.allow_affiliate AND p.affiliate_percent > 0
		    AND p.is_active AND p.deleted_at IS NULL`, code).Scan(&linkID); err != nil {
			fail(w, http.StatusNotFound, "link tidak ditemukan")
			return
		}
		// Dedup klik: IP yang sama hanya dihitung sekali per 24 jam per link —
		// tanpa ini spam klik menaikkan statistik afiliasi tanpa batas.
		ipHash := provider.HashKey(ipOf(r) + auth.Secret)
		var sudah int
		if db.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM affiliate_click
			  WHERE link_id=$1 AND ip_hash=$2 AND clicked_at > now() - interval '24 hours'`,
			linkID, ipHash).Scan(&sudah) == nil && sudah > 0 {
			ok(w, map[string]any{"ok": true, "deduped": true})
			return
		}
		if _, err := db.Exec(r.Context(),
			`INSERT INTO affiliate_click (link_id, ip_hash) VALUES ($1,$2)`,
			linkID, ipHash); err != nil {
			log.Printf("[WARN] catat klik afiliasi gagal: %v", err)
		}
		ok(w, map[string]any{"ok": true})
	}
}

// ------------------------------------------------------------------ KERANJANG

// Cart: GET /api/cart — harga dihitung ulang dari produk saat ini, bukan dari
// harga yang tersimpan di keranjang.
func Cart(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		rows, err := db.Query(r.Context(),
			`SELECT c.id, c.quantity, p.id, p.title, p.price, p.stock, p.is_active, p.is_demo, COALESCE(s.name,'')
			   FROM cart_item c
			   JOIN market_product p ON p.id = c.product_id
			   LEFT JOIN market_store s ON s.id = p.store_id
			  WHERE c.user_id=$1 ORDER BY c.created_at DESC`, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type item struct {
			ID        string  `json:"id"`
			ProductID string  `json:"product_id"`
			Title     string  `json:"title"`
			Store     string  `json:"store"`
			Price     float64 `json:"price"`
			Quantity  int     `json:"quantity"`
			Subtotal  float64 `json:"subtotal"`
			Tersedia  bool    `json:"tersedia"`
			Catatan   string  `json:"catatan,omitempty"`
		}
		var items []item
		var jml int
		var subtotal float64
		adaMasalah := false
		for rows.Next() {
			var x item
			var stock int
			var active, isDemo bool
			if err := rows.Scan(&x.ID, &x.Quantity, &x.ProductID, &x.Title, &x.Price, &stock, &active, &isDemo, &x.Store); err != nil {
				continue
			}
			x.Subtotal = x.Price * float64(x.Quantity)
			habis := stock > 0 && stock < x.Quantity
			x.Tersedia = active && !isDemo && stock != 0 && !habis
			switch {
			case isDemo:
				x.Catatan = "Produk demo tidak dapat dibeli"
			case !active:
				x.Catatan = "Produk sedang tidak ditayangkan"
			case habis:
				x.Catatan = "Stok tinggal " + itoa(stock)
			}
			if !x.Tersedia {
				adaMasalah = true
			} else {
				subtotal += x.Subtotal
			}
			jml += x.Quantity
			items = append(items, x)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[WARN] rows error in Cart: %v", err)
		}
		ok(w, map[string]any{
			// subtotal di level atas: halaman /keranjang memakainya; kalau tidak
			// ada, frontend menjumlahkan sendiri termasuk item yang tidak bisa dibeli.
			"subtotal": subtotal,
			"items":    items,
			"summary": map[string]any{
				"jumlah": jml, "subtotal": subtotal, "adaMasalah": adaMasalah,
			},
		})
	}
}

// AddToCart: produk yang sama menambah quantity lewat ON CONFLICT pada kunci
// unik (user_id, product_id), jadi dua permintaan bersamaan tidak bisa membuat
// dua baris produk yang sama.
func AddToCart(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			ProductID string `json:"product_id"`
			Quantity  num    `json:"quantity"`
		}
		if err := decode(r, &in); err != nil || in.ProductID == "" {
			fail(w, http.StatusBadRequest, "product_id wajib")
			return
		}
		add := in.Quantity.Int(1)
		if add < 1 {
			add = 1
		}

		var title string
		var stock int
		var active, isDemo bool
		var owner string
		if err := db.QueryRow(r.Context(),
			`SELECT p.title, p.stock, p.is_active, p.is_demo, COALESCE(s.user_id::text,'')
			   FROM market_product p LEFT JOIN market_store s ON s.id = p.store_id
			  WHERE p.id=$1 AND p.deleted_at IS NULL`,
			in.ProductID).Scan(&title, &stock, &active, &isDemo, &owner); err != nil {
			fail(w, http.StatusNotFound, "produk tidak ditemukan")
			return
		}
		if !active {
			fail(w, http.StatusBadRequest, "produk sedang tidak ditayangkan")
			return
		}
		if isDemo {
			fail(w, http.StatusBadRequest, "produk demo tidak dapat dibeli")
			return
		}
		// Penjual tidak bisa membeli produknya sendiri: uang bisa berputar balik
		// ke dirinya lewat keranjang.
		if owner == uid {
			fail(w, http.StatusBadRequest, "tidak bisa membeli produk sendiri")
			return
		}

		var qty int
		if err := db.QueryRow(r.Context(),
			`INSERT INTO cart_item (user_id, product_id, quantity) VALUES ($1,$2,$3)
			 ON CONFLICT (user_id, product_id) DO UPDATE SET quantity = cart_item.quantity + EXCLUDED.quantity, updated_at=now()
			 RETURNING quantity`, uid, in.ProductID, add).Scan(&qty); err != nil {
			log.Printf("[ERROR] tambah keranjang gagal: %v", err)
			fail(w, http.StatusInternalServerError, "gagal menambah keranjang")
			return
		}
		if stock > 0 && qty > stock {
			// Dikembalikan ke nilai sebelumnya supaya keranjang tidak menyimpan
			// jumlah yang tidak akan pernah bisa dibeli.
			if _, err := db.Exec(r.Context(),
				`UPDATE cart_item SET quantity = quantity - $1 WHERE user_id=$2 AND product_id=$3`,
				add, uid, in.ProductID); err != nil {
				log.Printf("[WARN] koreksi keranjang gagal: %v", err)
			}
			fail(w, http.StatusBadRequest, "stok hanya "+itoa(stock))
			return
		}
		ok(w, map[string]any{"ok": true, "quantity": qty})
	}
}

// UpdateCartItem: quantity 0 menghapus barisnya.
func UpdateCartItem(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in struct {
			Quantity num `json:"quantity"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		qty := in.Quantity.Int(-1)
		if qty < 0 {
			fail(w, http.StatusBadRequest, "quantity tidak sah")
			return
		}
		id := r.PathValue("id")
		if qty == 0 {
			res, err := db.Exec(r.Context(), `DELETE FROM cart_item WHERE id=$1 AND user_id=$2`, id, uid)
			if err != nil {
				fail(w, http.StatusInternalServerError, "gagal memperbarui keranjang")
				return
			}
			if res.RowsAffected() == 0 {
				fail(w, http.StatusNotFound, "baris keranjang tidak ditemukan")
				return
			}
			ok(w, map[string]any{"ok": true, "quantity": 0})
			return
		}
		// Syarat user_id ikut di dalam query: baris milik user lain tidak bisa diubah.
		res, err := db.Exec(r.Context(),
			`UPDATE cart_item c SET quantity=$1, updated_at=now()
			  WHERE c.id=$2 AND c.user_id=$3
			    AND (c.product_id IN (SELECT p.id FROM market_product p WHERE NOT p.is_demo AND (p.stock = 0 OR p.stock >= $1)))`,
			qty, id, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memperbarui keranjang")
			return
		}
		if res.RowsAffected() == 0 {
			fail(w, http.StatusBadRequest, "baris tidak ditemukan atau stok tidak mencukupi")
			return
		}
		ok(w, map[string]any{"ok": true, "quantity": qty})
	}
}

func DeleteCartItem(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		pID, valid := pathUUID(w, r, "id")
		if !valid {
			return
		}
		res, err := db.Exec(r.Context(), `DELETE FROM cart_item WHERE id=$1 AND user_id=$2`, pID, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal menghapus")
			return
		}
		if res.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "baris keranjang tidak ditemukan")
			return
		}
		ok(w, map[string]any{"ok": true})
	}
}
