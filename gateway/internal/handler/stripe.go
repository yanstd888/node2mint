package handler

// Jalur pembayaran Stripe (port dari web lama server/src/routes/payment.ts):
// - StripeCheckout: POST /api/wallet/stripe/checkout {amount} → URL Checkout
//   Session (mode=payment, IDR). Membuat credit_transaction PENDING dan
//   menempelkan id-nya sebagai client_reference_id sesi.
// - StripeWebhook: POST /api/webhooks/stripe → verifikasi tanda tangan
//   HMAC-SHA256 (stdlib), event checkout.session.completed → top-up jadi
//   COMPLETED + saldo bertambah. Idempoten: saldo hanya bertambah bila
//   UPDATE baris PENDING sukses (replay webhook tidak bisa mencetak uang).
//
// Tanpa SDK stripe-go: dua panggilan REST + satu verifikasi HMAC cukup, dan
// aturan proyek ini tanpa dependency baru.
//
// ponytail: cuma event checkout.session.completed yang ditangani; tambah
// refund/portal bila fiturnya benar-benar diminta.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const stripeAPI = "https://api.stripe.com/v1"

func stripeConfigured() bool { return os.Getenv("STRIPE_SECRET_KEY") != "" }

func StripeCheckout(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		if !stripeConfigured() {
			fail(w, http.StatusServiceUnavailable, "Pembayaran Stripe belum dikonfigurasi (kredensial kosong)")
			return
		}
		var in struct {
			Amount    num    `json:"amount"`
			ReturnURL string `json:"return_url"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		amount := in.Amount.Float()
		if amount < 10000 || amount > 50000000 {
			fail(w, http.StatusBadRequest, "nominal tidak valid (Rp10.000 – Rp50.000.000)")
			return
		}
		// Satu baris PENDING; webhook yang mengakuinya — sama seperti top-up
		// manual, tapi verifikatornya Stripe, bukan admin.
		var id string
		if err := db.QueryRow(r.Context(),
			`INSERT INTO credit_transaction (user_id, amount, type, status, description, reference_id)
			 VALUES ($1,$2,'TOPUP','PENDING','Top-up Stripe','stripe') RETURNING id`,
			uid, amount).Scan(&id); err != nil {
			log.Printf("[ERROR] stripe topup insert: %v", err)
			fail(w, http.StatusInternalServerError, "gagal membuat top-up")
			return
		}
		origin := "http://" + r.Host
		if r.TLS != nil {
			origin = "https://" + r.Host
		}
		// ponytail: origin dari Host; tambah whitelist domain bila nanti di-proxy.
		var email string
		_ = db.QueryRow(r.Context(), `SELECT COALESCE(email,'') FROM "user" WHERE id=$1`, uid).Scan(&email)
		returnURL := strings.TrimSpace(in.ReturnURL)
		if !strings.HasPrefix(returnURL, "/payment?") || strings.HasPrefix(returnURL, "//") {
			returnURL = "/wallet"
		}
		separator := "?"
		if strings.Contains(returnURL, "?") {
			separator = "&"
		}
		values := url.Values{
			"mode":                                   {"payment"},
			"payment_method_types[0]":                {"card"},
			"client_reference_id":                    {id},
			"customer_email":                         {email},
			"success_url":                            {origin + returnURL + separator + "stripe=success"},
			"cancel_url":                             {origin + returnURL + separator + "stripe=cancel"},
			"line_items[0][price_data][currency]":    {"idr"},
			"line_items[0][price_data][unit_amount]": {strconv.FormatInt(int64(amount), 10)},
			"line_items[0][price_data][product_data][name]": {"Pembayaran AI-sell"},
			"line_items[0][quantity]":                       {"1"},
			"metadata[tx_id]":                               {id},
		}
		params := strings.NewReader(values.Encode())
		req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, stripeAPI+"/checkout/sessions", params)
		req.Header.Set("Authorization", "Bearer "+os.Getenv("STRIPE_SECRET_KEY"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			fail(w, http.StatusBadGateway, "gagal menghubungi Stripe")
			return
		}
		defer res.Body.Close()
		var sesi struct {
			ID    string `json:"id"`
			URL   string `json:"url"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.NewDecoder(res.Body).Decode(&sesi); err != nil || !strings.HasPrefix(sesi.URL, "https://") {
			log.Printf("[ERROR] stripe session gagal (status %d): %v", res.StatusCode, sesi.Error)
			fail(w, http.StatusBadGateway, "gagal membuat sesi pembayaran")
			return
		}
		audit(r.Context(), db, uid, "TOPUP_STRIPE", id, nil, map[string]any{"amount": amount, "session": sesi.ID}, ipOf(r))
		ok(w, map[string]any{"url": sesi.URL, "id": id})
	}
}

// verifyStripeSignature: t=unix, v1=hex(hmac_sha256(secret, t+"."+body)).
// Menolak replay lebih dari 5 menit (rekomendasi Stripe).
func verifyStripeSignature(secret, header string, body []byte) bool {
	var t string
	var v1 []string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		if kv[0] == "t" {
			t = kv[1]
		}
		if kv[0] == "v1" {
			v1 = append(v1, kv[1])
		}
	}
	ts, err := strconv.ParseInt(t, 10, 64)
	if err != nil || time.Since(time.Unix(ts, 0)) > 5*time.Minute || len(v1) == 0 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(t + "."))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	for _, sig := range v1 {
		if hmac.Equal([]byte(sig), []byte(want)) {
			return true
		}
	}
	return false
}

func StripeWebhook(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
		if secret == "" {
			fail(w, http.StatusServiceUnavailable, "Webhook Stripe belum dikonfigurasi")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			fail(w, http.StatusBadRequest, "bad body")
			return
		}
		if !verifyStripeSignature(secret, r.Header.Get("Stripe-Signature"), body) {
			fail(w, http.StatusBadRequest, "tanda tangan tidak sah")
			return
		}
		var ev struct {
			Type string `json:"type"`
			Data struct {
				Object struct {
					ID                string `json:"id"`
					ClientReferenceID string `json:"client_reference_id"`
					PaymentStatus     string `json:"payment_status"`
				} `json:"object"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &ev); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		// Hanya sukses yang diakui; event lain di-200 supaya Stripe berhenti retry.
		if ev.Type != "checkout.session.completed" || ev.Data.Object.PaymentStatus != "paid" {
			w.WriteHeader(http.StatusOK)
			return
		}
		txID := ev.Data.Object.ClientReferenceID
		if txID == "" {
			txID = ev.Data.Object.ID
		}
		// Idempoten + jejak: COMPLETED hanya jika masih PENDING, lalu saldo
		// bertambah HANYA jika baris itu yang berubah. Satu transaksi.
		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal mulai transaksi")
			return
		}
		defer tx.Rollback(r.Context())
		tag, err := tx.Exec(r.Context(),
			`UPDATE credit_transaction SET status='COMPLETED'
			  WHERE id=$1 AND type='TOPUP' AND status='PENDING'`, txID)
		if err != nil {
			log.Printf("[ERROR] stripe webhook settle: %v", err)
			fail(w, http.StatusInternalServerError, "gagal mencatat top-up")
			return
		}
		credited := tag.RowsAffected() == 1
		if credited {
			// Saldo hanya bertambah bila baris ini yang baru saja berubah —
			// replay webhook tidak bisa mencetak uang dua kali.
			if _, err := tx.Exec(r.Context(),
				`UPDATE "user" u SET wallet_balance = wallet_balance + t.amount
				   FROM credit_transaction t WHERE t.id=$1 AND t.user_id = u.id`, txID); err != nil {
				log.Printf("[ERROR] stripe webhook settle: %v", err)
				fail(w, http.StatusInternalServerError, "gagal mencatat top-up")
				return
			}
			// Bonus undangan lunas juga di topup pertama lewat Stripe.
			var uidStripe string
			if err := tx.QueryRow(r.Context(),
				`SELECT user_id FROM credit_transaction WHERE id=$1`, txID).Scan(&uidStripe); err == nil {
				payDeferredReferralBonus(r.Context(), tx, uidStripe)
			}
		}
		if err := tx.Commit(r.Context()); err != nil {
			log.Printf("[ERROR] stripe webhook commit: %v", err)
			fail(w, http.StatusInternalServerError, "gagal mencatat top-up")
			return
		}
		if credited {
			log.Printf("[INFO] stripe top-up terbayar: tx=%s session=%s", txID, ev.Data.Object.ID)
		}
		w.WriteHeader(http.StatusOK)
	}
}
