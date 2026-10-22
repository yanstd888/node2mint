package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"
)

const cryptoInvoiceTTL = 30 * time.Minute

func cryptoIDRPerUSDT() (float64, bool) {
	n, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv("CRYPTO_IDR_PER_USDT")), 64)
	return n, err == nil && n > 0
}

func cryptoJSON(v any) ([]byte, error) { return json.Marshal(v) }

func cryptoHTTP(ctx context.Context, method, url string, body []byte, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	return raw, res.StatusCode, err
}

func cryptoCreateTransaction(ctx context.Context, db DB, uid, description string, amount float64) (string, error) {
	var id string
	err := db.QueryRow(ctx, `INSERT INTO credit_transaction (user_id, amount, type, status, description, reference_id) VALUES ($1,$2,'TOPUP','PENDING',$3,'crypto') RETURNING id`, uid, amount, description).Scan(&id)
	return id, err
}

func CryptoOnchainInvoice(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		address := strings.TrimSpace(os.Getenv("CRYPTO_TRON_ADDRESS"))
		rate, configured := cryptoIDRPerUSDT()
		if address == "" || !configured {
			fail(w, 503, "USDT TRC-20 belum dikonfigurasi")
			return
		}
		var in struct {
			Amount num `json:"amount"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, 400, "bad json")
			return
		}
		idr := in.Amount.Float()
		if idr < 10000 || idr > 50000000 {
			fail(w, 400, "nominal tidak valid")
			return
		}
		txID, err := cryptoCreateTransaction(r.Context(), db, uidFromCtx(r.Context()), "Top-up USDT TRC-20", idr)
		if err != nil {
			fail(w, 500, "gagal membuat invoice crypto")
			return
		}
		// Micro-unit suffix prevents identical invoices from being indistinguishable on one address.
		suffix := new(big.Int)
		suffix.SetString(strings.ReplaceAll(txID, "-", "")[28:32], 16)
		expected := idr/rate + float64(suffix.Int64()%1000+1)/1_000_000
		expires := time.Now().Add(cryptoInvoiceTTL)
		var paymentID string
		if err := db.QueryRow(r.Context(), `INSERT INTO crypto_payment (user_id, credit_transaction_id, provider, asset, network, expected_amount, destination, expires_at) VALUES ($1,$2,'TRON_TRC20','USDT','TRC20',$3,$4,$5) RETURNING id`, uidFromCtx(r.Context()), txID, expected, address, expires).Scan(&paymentID); err != nil {
			fail(w, 500, "gagal menyimpan invoice crypto")
			return
		}
		ok(w, map[string]any{"invoice_id": paymentID, "provider": "TRON_TRC20", "asset": "USDT", "network": "TRC20", "address": address, "amount_usdt": fmt.Sprintf("%.6f", expected), "expires_at": expires.UTC().Format(time.RFC3339)})
	}
}

func CryptoStatus(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		uid := uidFromCtx(r.Context())
		var status, provider, asset, network, address, amount, txHash string
		err := db.QueryRow(r.Context(), `SELECT status, provider, asset, COALESCE(network,''), COALESCE(destination,''), expected_amount::text, COALESCE(tx_hash,'') FROM crypto_payment WHERE id=$1 AND user_id=$2`, id, uid).Scan(&status, &provider, &asset, &network, &address, &amount, &txHash)
		if err != nil {
			fail(w, 404, "invoice crypto tidak ditemukan")
			return
		}
		ok(w, map[string]any{"status": status, "provider": provider, "asset": asset, "network": network, "address": address, "amount": amount, "tx_hash": txHash})
	}
}

func CryptoQRCode(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var address string
		if err := db.QueryRow(r.Context(), `SELECT destination FROM crypto_payment WHERE id=$1 AND user_id=$2`, chi.URLParam(r, "id"), uidFromCtx(r.Context())).Scan(&address); err != nil || address == "" {
			fail(w, http.StatusNotFound, "invoice crypto tidak ditemukan")
			return
		}
		png, err := qrcode.Encode(address, qrcode.Medium, 256)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal membuat QR")
			return
		}
		webp, err := convertToWebP(png, true)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal mengenkode QR")
			return
		}
		w.Header().Set("Content-Type", "image/webp")
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = w.Write(webp)
	}
}

func settleCryptoPayment(ctx context.Context, db DB, paymentID, txHash string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var txID, userID string
	if err := tx.QueryRow(ctx, `SELECT credit_transaction_id, user_id FROM crypto_payment WHERE id=$1 AND status='PENDING' AND expires_at > now() FOR UPDATE`, paymentID).Scan(&txID, &userID); err != nil {
		return err
	}
	res, err := tx.Exec(ctx, `UPDATE crypto_payment SET status='PAID', tx_hash=COALESCE(NULLIF($1,''),tx_hash), paid_at=now() WHERE id=$2 AND status='PENDING'`, txHash, paymentID)
	if err != nil || res.RowsAffected() != 1 {
		return fmt.Errorf("crypto payment already settled")
	}
	res, err = tx.Exec(ctx, `UPDATE credit_transaction SET status='COMPLETED' WHERE id=$1 AND status='PENDING'`, txID)
	if err != nil || res.RowsAffected() != 1 {
		return fmt.Errorf("credit transaction already settled")
	}
	if _, err = tx.Exec(ctx, `UPDATE "user" u SET wallet_balance=u.wallet_balance+c.amount FROM credit_transaction c WHERE c.id=$1 AND c.user_id=u.id`, txID); err != nil {
		return err
	}
	payDeferredReferralBonus(ctx, tx, userID)
	if err := enqueueEmailOutbox(ctx, tx, "crypto:"+paymentID, emailEventUSDT, userID, paymentID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func StartCryptoWatcher(db DB, interval time.Duration) {
	watchTRON := os.Getenv("CRYPTO_TRON_ADDRESS") != "" && os.Getenv("CRYPTO_TRONGRID_API_KEY") != "" && os.Getenv("CRYPTO_USDT_CONTRACT") != ""
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, _ = db.Exec(ctx, `UPDATE crypto_payment SET status='EXPIRED' WHERE status='PENDING' AND expires_at <= now()`)
			cancel()
			if watchTRON {
				cryptoScanTRON(db)
			}
			<-ticker.C
		}
	}()
}

func cryptoScanTRON(db DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var address string
	if err := db.QueryRow(ctx, `SELECT destination FROM crypto_payment WHERE provider='TRON_TRC20' AND status='PENDING' AND expires_at>now() ORDER BY created_at LIMIT 1`).Scan(&address); err != nil {
		return
	}
	endpoint := "https://api.trongrid.io/v1/accounts/" + url.PathEscape(address) + "/transactions/trc20"
	query := url.Values{"limit": {"200"}, "only_confirmed": {"true"}, "contract_address": {os.Getenv("CRYPTO_USDT_CONTRACT")}}
	raw, code, err := cryptoHTTP(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil, map[string]string{"TRON-PRO-API-KEY": os.Getenv("CRYPTO_TRONGRID_API_KEY")})
	if err != nil || code != 200 {
		return
	}
	var out struct {
		Data []struct {
			TransactionID string `json:"transaction_id"`
			To            string `json:"to"`
			Value         string `json:"value"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &out) != nil {
		return
	}
	for _, item := range out.Data {
		if !strings.EqualFold(item.To, address) {
			continue
		}
		value, ok := new(big.Float).SetString(item.Value)
		if !ok {
			continue
		}
		amount := new(big.Float).Quo(value, big.NewFloat(1_000_000))
		f, _ := amount.Float64()
		var paymentID string
		if db.QueryRow(ctx, `SELECT id FROM crypto_payment WHERE provider='TRON_TRC20' AND status='PENDING' AND tx_hash IS NULL AND destination=$1 AND expected_amount = $2 AND expires_at > now() ORDER BY created_at LIMIT 1`, address, f).Scan(&paymentID) == nil {
			_ = settleCryptoPayment(ctx, db, paymentID, item.TransactionID)
		}
	}
}
