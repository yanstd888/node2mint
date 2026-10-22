package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	dokuSandboxAPI       = "https://api-sandbox.doku.com"
	dokuProductionAPI    = "https://api.doku.com"
	dokuCheckoutPath     = "/checkout/v1/payment"
	dokuOrderStatusPath  = "/orders/v1/status/"
	dokuWebhookPath      = "/api/webhooks/doku"
	dokuInvoicePrefix    = "N2M"
	dokuPaymentDueMinute = 30
)

type dokuOrder struct {
	InvoiceNumber string `json:"invoice_number"`
	Amount        num    `json:"amount"`
	Currency      string `json:"currency"`
	Status        string `json:"status"`
}

type dokuTransaction struct {
	Status            string `json:"status"`
	OriginalRequestID string `json:"original_request_id"`
}

type dokuStatusResponse struct {
	Order       dokuOrder       `json:"order"`
	Transaction dokuTransaction `json:"transaction"`
}

type dokuCheckoutResponse struct {
	Response struct {
		Order   dokuOrder `json:"order"`
		Payment struct {
			URL string `json:"url"`
		} `json:"payment"`
	} `json:"response"`
}

type dokuHTTPError struct {
	operation string
	status    int
}

func (e *dokuHTTPError) Error() string {
	return fmt.Sprintf("doku %s http %d", e.operation, e.status)
}

// Only a clear client-side rejection proves no provider invoice was created.
// A timeout, conflict, rate limit, or 5xx remains CREATING for reconciliation.
func dokuCreateRejected(err error) bool {
	var providerErr *dokuHTTPError
	return errors.As(err, &providerErr) && providerErr.operation == "create payment" &&
		providerErr.status >= http.StatusBadRequest && providerErr.status < http.StatusInternalServerError &&
		providerErr.status != http.StatusRequestTimeout && providerErr.status != http.StatusConflict && providerErr.status != http.StatusTooManyRequests
}

func dokuStatusNotFound(err error) bool {
	var providerErr *dokuHTTPError
	return errors.As(err, &providerErr) && providerErr.operation == "check status" && providerErr.status == http.StatusNotFound
}

func dokuClientID() string  { return strings.TrimSpace(os.Getenv("DOKU_CLIENT_ID")) }
func dokuSecretKey() string { return strings.TrimSpace(os.Getenv("DOKU_SECRET_KEY")) }

func dokuEnvironment() string {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("DOKU_ENV")), "production") {
		return "production"
	}
	return "sandbox"
}

func dokuAPIBase() string {
	if dokuEnvironment() == "production" {
		return dokuProductionAPI
	}
	return dokuSandboxAPI
}

func dokuConfigured() bool { return dokuClientID() != "" && dokuSecretKey() != "" }

// DOKU Non-SNAP signs one canonical header component per line. Digest applies
// only to POST bodies; notifications use the merchant notification URL path.
func dokuSignature(clientID, requestID, timestamp, requestTarget string, body []byte, secret string) string {
	lines := []string{
		"Client-Id:" + clientID,
		"Request-Id:" + requestID,
		"Request-Timestamp:" + timestamp,
		"Request-Target:" + requestTarget,
	}
	if body != nil {
		digest := sha256.Sum256(body)
		lines = append(lines, "Digest:"+base64.StdEncoding.EncodeToString(digest[:]))
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.Join(lines, "\n")))
	return "HMACSHA256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func validDokuSignature(clientID, requestID, timestamp, requestTarget, signature, secret string, body []byte) bool {
	if clientID == "" || requestID == "" || timestamp == "" || requestTarget == "" || secret == "" {
		return false
	}
	want := dokuSignature(clientID, requestID, timestamp, requestTarget, body, secret)
	return hmac.Equal([]byte(strings.TrimSpace(signature)), []byte(want))
}

func dokuRequestID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return hex.EncodeToString(b[:4]) + "-" + hex.EncodeToString(b[4:6]) + "-" + hex.EncodeToString(b[6:8]) + "-" + hex.EncodeToString(b[8:10]) + "-" + hex.EncodeToString(b[10:]), nil
}

// dokuInvoiceNumber stays below the 30-character credit-card limit while
// retaining 96 bits from the internal UUID. It is never user-controlled.
func dokuInvoiceNumber(creditID string) string {
	out := make([]byte, 0, 24)
	for _, c := range creditID {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			out = append(out, byte(c))
			if len(out) == 24 {
				break
			}
		}
	}
	return dokuInvoicePrefix + string(out)
}

func validDokuInvoiceNumber(invoice string) bool {
	if len(invoice) < len(dokuInvoicePrefix)+1 || len(invoice) > 30 || !strings.HasPrefix(invoice, dokuInvoicePrefix) {
		return false
	}
	for _, c := range invoice {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

func dokuPaymentStatus(orderStatus, transactionStatus string) string {
	if strings.EqualFold(strings.TrimSpace(orderStatus), "ORDER_EXPIRED") {
		return "EXPIRED"
	}
	switch strings.ToUpper(strings.TrimSpace(transactionStatus)) {
	case "SUCCESS":
		return "PAID"
	case "EXPIRED":
		return "EXPIRED"
	case "FAILED", "CANCELLED", "CANCELED":
		return "FAILED"
	default:
		return "PENDING"
	}
}

func dokuAmountAccepted(expected, received float64) bool {
	return expected > 0 && !math.IsNaN(received) && !math.IsInf(received, 0) && received >= expected
}

func dokuPublicHTTPSBase() string {
	u, err := url.Parse(strings.TrimSpace(os.Getenv("PUBLIC_WEB_URL")))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ""
	}
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}

func dokuFinishURL(base, finishPath, invoice string) string {
	query := url.Values{}
	if u, err := url.Parse(finishPath); err == nil && u.Scheme == "" && u.Host == "" && u.Path == "/payment" {
		for _, key := range []string{"kind", "id", "qty", "amount", "label"} {
			if value := u.Query().Get(key); value != "" {
				query.Set(key, value)
			}
		}
	}
	query.Set("doku", "finish")
	query.Set("doku_invoice", invoice)
	return base + "/payment?" + query.Encode()
}

func dokuCheckoutURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Scheme == "https" && u.Host != ""
}

func dokuCreatePayment(ctx context.Context, requestID string, body []byte) (dokuCheckoutResponse, error) {
	var out dokuCheckoutResponse
	clientID, secret := dokuClientID(), dokuSecretKey()
	if clientID == "" || secret == "" {
		return out, errors.New("doku belum dikonfigurasi")
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dokuAPIBase()+dokuCheckoutPath, bytes.NewReader(body))
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Client-Id", clientID)
	req.Header.Set("Request-Id", requestID)
	req.Header.Set("Request-Timestamp", timestamp)
	req.Header.Set("Signature", dokuSignature(clientID, requestID, timestamp, dokuCheckoutPath, body, secret))

	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return out, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return out, err
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return out, &dokuHTTPError{operation: "create payment", status: res.StatusCode}
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("doku create payment response: %w", err)
	}
	if !dokuCheckoutURL(out.Response.Payment.URL) {
		return out, errors.New("doku create payment missing checkout url")
	}
	return out, nil
}

func dokuCheckStatus(ctx context.Context, invoice string) (dokuStatusResponse, []byte, error) {
	var out dokuStatusResponse
	requestID, err := dokuRequestID()
	if err != nil {
		return out, nil, err
	}
	clientID, secret := dokuClientID(), dokuSecretKey()
	if clientID == "" || secret == "" {
		return out, nil, errors.New("doku belum dikonfigurasi")
	}
	target := dokuOrderStatusPath + url.PathEscape(invoice)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dokuAPIBase()+target, nil)
	if err != nil {
		return out, nil, err
	}
	req.Header.Set("Client-Id", clientID)
	req.Header.Set("Request-Id", requestID)
	req.Header.Set("Request-Timestamp", timestamp)
	req.Header.Set("Signature", dokuSignature(clientID, requestID, timestamp, target, nil, secret))

	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return out, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return out, nil, err
	}
	if res.StatusCode != http.StatusOK {
		return out, nil, &dokuHTTPError{operation: "check status", status: res.StatusCode}
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, nil, fmt.Errorf("doku status response: %w", err)
	}
	if out.Order.InvoiceNumber != invoice {
		return out, nil, errors.New("doku status invoice tidak cocok")
	}
	return out, raw, nil
}

func StartDokuReconciler(db DB, interval time.Duration) {
	if !dokuConfigured() {
		return
	}
	go func() {
		for {
			reconcileDoku(context.Background(), db)
			time.Sleep(interval)
		}
	}()
}

func reconcileDoku(ctx context.Context, db DB) {
	rows, err := db.Query(ctx, `SELECT invoice_number,status,
		created_at < now() - interval '2 minutes' AS stale_creation
		FROM doku_payment WHERE status IN ('CREATING','PENDING')
		ORDER BY created_at LIMIT 100`)
	if err != nil {
		log.Printf("[ERROR] doku reconcile list: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var invoice, status string
		var staleCreation bool
		if rows.Scan(&invoice, &status, &staleCreation) != nil {
			continue
		}
		in, raw, err := dokuCheckStatus(ctx, invoice)
		if err != nil {
			if status == "CREATING" && staleCreation && dokuStatusNotFound(err) {
				markDokuCreationFailed(ctx, db, invoice)
			} else {
				log.Printf("[WARN] doku reconcile %s: %v", invoice, err)
			}
			continue
		}
		if code, message := settleDoku(ctx, db, in, "RECONCILE", raw); code >= http.StatusInternalServerError {
			log.Printf("[ERROR] doku reconcile %s: %s", invoice, message)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("[ERROR] doku reconcile rows: %v", err)
	}
}

func DokuCheckout(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !dokuConfigured() {
			fail(w, http.StatusServiceUnavailable, "DOKU Checkout belum dikonfigurasi")
			return
		}
		webBase := dokuPublicHTTPSBase()
		if webBase == "" {
			fail(w, http.StatusServiceUnavailable, "PUBLIC_WEB_URL HTTPS belum dikonfigurasi")
			return
		}

		var in struct {
			Amount     num    `json:"amount"`
			FinishPath string `json:"finish_path"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		amount := in.Amount.Float()
		if !(amount >= 10000 && amount <= 50000000) || math.Trunc(amount) != amount {
			fail(w, http.StatusBadRequest, "nominal harus berupa Rupiah bulat (Rp10.000 – Rp50.000.000)")
			return
		}
		requestKey, valid := idempotencyKey(w, r)
		if !valid {
			return
		}
		uid := uidFromCtx(r.Context())

		var existingInvoice, existingStatus, existingURL, existingEnvironment string
		var existingAmount float64
		err := db.QueryRow(r.Context(), `SELECT invoice_number,status,amount,COALESCE(checkout_url,''),environment
			FROM doku_payment WHERE user_id=$1 AND status IN ('CREATING','PENDING')
			ORDER BY created_at DESC LIMIT 1`, uid).
			Scan(&existingInvoice, &existingStatus, &existingAmount, &existingURL, &existingEnvironment)
		if err == nil {
			if existingAmount != amount {
				fail(w, http.StatusConflict, "selesaikan tagihan DOKU sebelumnya sebelum mengubah nominal")
				return
			}
			ok(w, dokuPendingPayload(existingInvoice, existingStatus, existingURL, existingEnvironment))
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[ERROR] baca invoice DOKU pending: %v", err)
			fail(w, http.StatusInternalServerError, "gagal memeriksa pembayaran")
			return
		}

		var legacyPending bool
		if err := db.QueryRow(r.Context(),
			`SELECT EXISTS (SELECT 1 FROM credit_transaction
			  WHERE user_id=$1 AND type='TOPUP' AND status='PENDING')`, uid).Scan(&legacyPending); err != nil {
			log.Printf("[ERROR] cek top-up pending: %v", err)
			fail(w, http.StatusInternalServerError, "gagal memeriksa pembayaran")
			return
		}
		if legacyPending {
			fail(w, http.StatusConflict, "masih ada pembayaran sebelumnya yang menunggu")
			return
		}

		err = db.QueryRow(r.Context(), `SELECT invoice_number,status,amount,COALESCE(checkout_url,''),environment
			FROM doku_payment WHERE user_id=$1 AND request_key=$2`, uid, requestKey).
			Scan(&existingInvoice, &existingStatus, &existingAmount, &existingURL, &existingEnvironment)
		if err == nil {
			if existingAmount != amount {
				fail(w, http.StatusUnprocessableEntity, "Idempotency-Key tidak boleh dipakai untuk nominal berbeda")
				return
			}
			if existingStatus == "CREATING" || existingStatus == "PENDING" {
				ok(w, dokuPendingPayload(existingInvoice, existingStatus, existingURL, existingEnvironment))
				return
			}
			fail(w, http.StatusConflict, "tagihan sebelumnya sudah berakhir; mulai pembayaran baru")
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[ERROR] baca DOKU idempotency: %v", err)
			fail(w, http.StatusInternalServerError, "gagal memeriksa pembayaran")
			return
		}

		requestID, err := dokuRequestID()
		if err != nil {
			log.Printf("[ERROR] buat request id DOKU: %v", err)
			fail(w, http.StatusInternalServerError, "gagal membuat pembayaran")
			return
		}
		environment := dokuEnvironment()
		invoice := dokuInvoiceNumber(requestID)
		if !validDokuInvoiceNumber(invoice) {
			fail(w, http.StatusInternalServerError, "gagal membuat invoice")
			return
		}
		finishURL := dokuFinishURL(webBase, in.FinishPath, invoice)
		payload, err := json.Marshal(map[string]any{
			"order": map[string]any{
				"amount":              int64(amount),
				"invoice_number":      invoice,
				"currency":            "IDR",
				"callback_url":        finishURL,
				"callback_url_result": finishURL,
				"language":            "ID",
				"auto_redirect":       true,
			},
			"payment": map[string]any{"payment_due_date": dokuPaymentDueMinute},
			"additional_info": map[string]any{
				"override_notification_url": webBase + dokuWebhookPath,
			},
		})
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal membuat pembayaran")
			return
		}

		// Simpan intent lebih dulu: reload/race tidak boleh membuat dua invoice
		// provider hidup atau kehilangan ikatan ledger jika DOKU lambat merespons.
		tx, err := db.Begin(r.Context())
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal membuat pembayaran")
			return
		}
		defer tx.Rollback(r.Context())
		var creditID string
		description := "Top-up DOKU Sandbox"
		if environment == "production" {
			description = "Top-up DOKU Production"
		}
		if err := tx.QueryRow(r.Context(), `INSERT INTO credit_transaction (user_id,amount,type,status,description,reference_id,environment)
			VALUES ($1,$2,'TOPUP','PENDING',$3,'doku',$4) RETURNING id`, uid, amount, description, environment).Scan(&creditID); err != nil {
			log.Printf("[ERROR] doku topup insert: %v", err)
			fail(w, http.StatusInternalServerError, "gagal membuat top-up")
			return
		}
		var inserted string
		err = tx.QueryRow(r.Context(), `INSERT INTO doku_payment
			(user_id,credit_transaction_id,invoice_number,request_key,request_id,amount,currency,status,environment,expires_at)
			VALUES ($1,$2,$3,$4,$5,$6,'IDR','CREATING',$7,now() + ($8 * interval '1 minute'))
			ON CONFLICT DO NOTHING
			RETURNING invoice_number`, uid, creditID, invoice, requestKey, requestID, amount, environment, dokuPaymentDueMinute).Scan(&inserted)
		if errors.Is(err, pgx.ErrNoRows) {
			lookupErr := tx.QueryRow(r.Context(), `SELECT invoice_number,status,amount,COALESCE(checkout_url,''),environment
				FROM doku_payment WHERE user_id=$1 AND status IN ('CREATING','PENDING') ORDER BY created_at DESC LIMIT 1`, uid).
				Scan(&existingInvoice, &existingStatus, &existingAmount, &existingURL, &existingEnvironment)
			if lookupErr == nil {
				if existingAmount != amount {
					fail(w, http.StatusConflict, "selesaikan tagihan DOKU sebelumnya sebelum mengubah nominal")
					return
				}
				ok(w, dokuPendingPayload(existingInvoice, existingStatus, existingURL, existingEnvironment))
				return
			}
			log.Printf("[ERROR] doku payment conflict without active invoice: %v", lookupErr)
			fail(w, http.StatusConflict, "tagihan sebelumnya sudah berakhir; mulai pembayaran baru")
			return
		}
		if err != nil {
			log.Printf("[ERROR] doku payment insert: %v", err)
			fail(w, http.StatusInternalServerError, "gagal menyimpan pembayaran")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan pembayaran")
			return
		}

		created, err := dokuCreatePayment(r.Context(), requestID, payload)
		if err != nil || created.Response.Order.InvoiceNumber != invoice || created.Response.Order.Amount.Float() != amount ||
			(created.Response.Order.Currency != "" && !strings.EqualFold(created.Response.Order.Currency, "IDR")) {
			if err != nil && dokuCreateRejected(err) {
				log.Printf("[WARN] doku create payment ditolak %s: %v", invoice, err)
				markDokuCreationFailed(r.Context(), db, invoice)
				fail(w, http.StatusBadGateway, "DOKU menolak pembayaran")
				return
			}
			if err != nil {
				log.Printf("[WARN] doku create payment belum pasti %s: %v", invoice, err)
			} else {
				log.Printf("[WARN] doku create payment response mismatch: %s", invoice)
			}
			// ponytail: Check Status tidak mengembalikan payment.url; tahan intent
			// aktif sampai status provider pasti daripada menerbitkan invoice kedua.
			ok(w, dokuPendingPayload(invoice, "CREATING", "", environment))
			return
		}
		tag, err := db.Exec(r.Context(), `UPDATE doku_payment SET checkout_url=$1,status='PENDING'
			WHERE invoice_number=$2 AND user_id=$3 AND status='CREATING'`, created.Response.Payment.URL, invoice, uid)
		if err != nil || tag.RowsAffected() != 1 {
			if err != nil {
				log.Printf("[ERROR] simpan URL DOKU: %v", err)
			} else {
				log.Printf("[ERROR] status DOKU berubah sebelum URL tersimpan: %s", invoice)
			}
			ok(w, dokuPendingPayload(invoice, "CREATING", "", environment))
			return
		}
		audit(r.Context(), db, uid, "TOPUP_DOKU", creditID, nil, map[string]any{"amount": amount, "invoice_number": invoice}, ipOf(r))
		ok(w, map[string]any{"id": creditID, "invoice_number": invoice, "checkout_url": created.Response.Payment.URL, "status": "PENDING", "environment": environment})
	}
}

func markDokuCreationFailed(ctx context.Context, db DB, invoice string) {
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("[ERROR] mulai rollback DOKU: %v", err)
		return
	}
	defer tx.Rollback(ctx)
	var creditID string
	if err := tx.QueryRow(ctx, `UPDATE doku_payment SET status='FAILED'
		WHERE invoice_number=$1 AND status='CREATING'
		RETURNING credit_transaction_id`, invoice).Scan(&creditID); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[ERROR] tandai invoice DOKU gagal: %v", err)
		}
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE credit_transaction SET status='FAILED' WHERE id=$1 AND status='PENDING'`, creditID); err != nil {
		log.Printf("[ERROR] tandai credit DOKU gagal: %v", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("[ERROR] simpan gagal DOKU: %v", err)
	}
}

func dokuPendingPayload(invoice, status, checkoutURL, environment string) map[string]any {
	out := map[string]any{"invoice_number": invoice, "status": status, "pending": true, "environment": environment}
	if status == "CREATING" {
		out["creating"] = true
	}
	if checkoutURL != "" {
		out["checkout_url"] = checkoutURL
	}
	return out
}

// DokuPending restores the newest live invoice after a reload. It is owner scoped.
func DokuPending(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var invoice, status, checkoutURL, environment string
		err := db.QueryRow(r.Context(), `SELECT invoice_number,status,COALESCE(checkout_url,''),environment
			FROM doku_payment WHERE user_id=$1 AND status IN ('CREATING','PENDING')
			ORDER BY created_at DESC LIMIT 1`, uidFromCtx(r.Context())).
			Scan(&invoice, &status, &checkoutURL, &environment)
		if errors.Is(err, pgx.ErrNoRows) {
			ok(w, map[string]any{})
			return
		}
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal memeriksa pembayaran")
			return
		}
		ok(w, dokuPendingPayload(invoice, status, checkoutURL, environment))
	}
}

// DokuStatus is local invoice state only; browser redirects never settle funds.
func DokuStatus(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		invoice := r.PathValue("invoiceNumber")
		if !validDokuInvoiceNumber(invoice) {
			fail(w, http.StatusBadRequest, "invoice DOKU tidak valid")
			return
		}
		var status, checkoutURL string
		if err := db.QueryRow(r.Context(), `SELECT status,COALESCE(checkout_url,'') FROM doku_payment WHERE invoice_number=$1 AND user_id=$2`, invoice, uidFromCtx(r.Context())).Scan(&status, &checkoutURL); err != nil {
			fail(w, http.StatusNotFound, "pembayaran DOKU tidak ditemukan")
			return
		}
		out := map[string]any{"status": status}
		if checkoutURL != "" {
			out["checkout_url"] = checkoutURL
		}
		ok(w, out)
	}
}

func DokuWebhook(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID, secret := dokuClientID(), dokuSecretKey()
		if clientID == "" || secret == "" {
			fail(w, http.StatusServiceUnavailable, "Webhook DOKU belum dikonfigurasi")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			fail(w, http.StatusBadRequest, "bad body")
			return
		}
		requestTarget := r.URL.EscapedPath()
		if requestTarget != dokuWebhookPath || r.Header.Get("Client-Id") != clientID ||
			!validDokuSignature(clientID, r.Header.Get("Request-Id"), r.Header.Get("Request-Timestamp"), requestTarget, r.Header.Get("Signature"), secret, body) {
			fail(w, http.StatusBadRequest, "notifikasi DOKU tidak sah")
			return
		}
		var in dokuStatusResponse
		if json.Unmarshal(body, &in) != nil || !validDokuInvoiceNumber(in.Order.InvoiceNumber) {
			fail(w, http.StatusBadRequest, "notifikasi DOKU tidak valid")
			return
		}
		status, message := settleDoku(r.Context(), db, in, "WEBHOOK", body)
		if message != "" {
			fail(w, status, message)
			return
		}
		w.WriteHeader(status)
	}
}

func settleDoku(ctx context.Context, db DB, in dokuStatusResponse, source string, raw []byte) (int, string) {
	paymentState := dokuPaymentStatus(in.Order.Status, in.Transaction.Status)
	tx, err := db.Begin(ctx)
	if err != nil {
		return http.StatusInternalServerError, "gagal memulai transaksi"
	}
	defer tx.Rollback(ctx)

	var paymentID, creditID, userID, environment, requestID, currency string
	var expected float64
	if err := tx.QueryRow(ctx, `SELECT id,credit_transaction_id,user_id,amount,environment,request_id,currency
		FROM doku_payment WHERE invoice_number=$1 FOR UPDATE`, in.Order.InvoiceNumber).
		Scan(&paymentID, &creditID, &userID, &expected, &environment, &requestID, &currency); err != nil {
		return http.StatusOK, "" // unknown/expired callbacks must not retry forever
	}
	if in.Order.Currency != "" && !strings.EqualFold(in.Order.Currency, currency) {
		return http.StatusBadRequest, "mata uang DOKU tidak cocok"
	}
	if paymentState == "PAID" {
		if in.Transaction.OriginalRequestID == "" || in.Transaction.OriginalRequestID != requestID {
			return http.StatusBadRequest, "request DOKU tidak cocok"
		}
		if !dokuAmountAccepted(expected, in.Order.Amount.Float()) {
			return http.StatusBadRequest, "nominal DOKU kurang dari invoice"
		}
	}
	if err := recordDokuPaymentEvent(ctx, tx, paymentID, source, paymentState, raw); err != nil {
		return http.StatusInternalServerError, "gagal mencatat notifikasi DOKU"
	}

	switch paymentState {
	case "PENDING":
		if _, err := tx.Exec(ctx, `UPDATE doku_payment SET status='PENDING'
			WHERE id=$1 AND status='CREATING'`, paymentID); err != nil {
			return http.StatusInternalServerError, "gagal memperbarui status DOKU"
		}
	case "PAID":
		tag, err := tx.Exec(ctx, `UPDATE credit_transaction SET status='COMPLETED'
			WHERE id=$1 AND type='TOPUP' AND status='PENDING'`, creditID)
		if err != nil {
			return http.StatusInternalServerError, "gagal mencatat top-up"
		}
		if tag.RowsAffected() == 1 {
			if err := enqueueEmailOutbox(ctx, tx, "doku:"+in.Order.InvoiceNumber, emailEventDoku, userID, in.Order.InvoiceNumber); err != nil {
				return http.StatusInternalServerError, "gagal menjadwalkan email pembayaran"
			}
			if environment == "production" {
				if _, err := tx.Exec(ctx, `UPDATE "user" u SET wallet_balance=u.wallet_balance+c.amount
					FROM credit_transaction c WHERE c.id=$1 AND c.user_id=u.id`, creditID); err != nil {
					return http.StatusInternalServerError, "gagal mengkredit wallet"
				}
				payDeferredReferralBonus(ctx, tx, userID)
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE doku_payment SET status='PAID',paid_at=now()
			WHERE id=$1 AND status IN ('CREATING','PENDING')`, paymentID); err != nil {
			return http.StatusInternalServerError, "gagal mencatat pembayaran"
		}
	case "FAILED", "EXPIRED":
		if _, err := tx.Exec(ctx, `UPDATE credit_transaction SET status='FAILED' WHERE id=$1 AND status='PENDING'`, creditID); err != nil {
			return http.StatusInternalServerError, "gagal memperbarui pembayaran"
		}
		if _, err := tx.Exec(ctx, `UPDATE doku_payment SET status=$1 WHERE id=$2 AND status IN ('CREATING','PENDING')`, paymentState, paymentID); err != nil {
			return http.StatusInternalServerError, "gagal memperbarui pembayaran"
		}
	}
	if err := auditTx(ctx, tx, userID, "TOPUP_DOKU", in.Order.InvoiceNumber, map[string]any{"status": "PENDING", "amount": expected}, map[string]any{"status": paymentState, "amount": in.Order.Amount.Float(), "environment": environment}, ""); err != nil {
		return http.StatusInternalServerError, "gagal mencatat audit pembayaran"
	}
	if err := tx.Commit(ctx); err != nil {
		return http.StatusInternalServerError, "gagal menyelesaikan pembayaran"
	}
	return http.StatusOK, ""
}

func recordDokuPaymentEvent(ctx context.Context, tx pgx.Tx, paymentID, source, status string, raw []byte) error {
	if !json.Valid(raw) {
		raw = []byte(`{}`)
	}
	digest := sha256.Sum256(raw)
	_, err := tx.Exec(ctx, `INSERT INTO doku_payment_event(payment_id,source,event_status,payload_hash,payload)
		VALUES ($1,$2,$3,$4,$5::jsonb) ON CONFLICT (payment_id,source,payload_hash) DO NOTHING`,
		paymentID, source, status, hex.EncodeToString(digest[:]), string(raw))
	return err
}
