package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	emailEventDoku   = "DOKU_TOPUP"
	emailEventManual = "MANUAL_TOPUP"
	emailEventUSDT   = "USDT_TRC20_TOPUP"
	emailEventMarket = "MARKET_ORDER"
	emailMaxAttempts = 3
	emailTelegramURL = "https://t.me/node2mint"
)

// SendPulse endpoints. Package vars so tests can point them at a stub server.
var (
	sendpulseTokenEndpoint = "https://api.sendpulse.com/oauth/access_token"
	sendpulseEmailEndpoint = "https://api.sendpulse.com/smtp/emails"
)

func sendpulseClientID() string     { return strings.TrimSpace(os.Getenv("SENDPULSE_CLIENT_ID")) }
func sendpulseClientSecret() string { return strings.TrimSpace(os.Getenv("SENDPULSE_CLIENT_SECRET")) }
func emailSenderEmail() string      { return strings.TrimSpace(os.Getenv("EMAIL_SENDER_EMAIL")) }

func emailSenderName() string {
	if name := strings.TrimSpace(os.Getenv("EMAIL_SENDER_NAME")); name != "" {
		return name
	}
	return "Node2Mint"
}

// tokenCache holds one SendPulse OAuth token for the process. Tokens live 3600s;
// the mutex serialises refreshes so a burst of outbox rows cannot stampede the
// OAuth endpoint.
//
// ponytail: process-local token, fine for a single gateway replica. Move to
// Redis if the gateway is ever scaled horizontally.
var tokenCache struct {
	sync.Mutex
	token   string
	expires time.Time
}

func sendpulseAccessToken(ctx context.Context) (string, error) {
	id, secret := sendpulseClientID(), sendpulseClientSecret()
	if id == "" || secret == "" {
		return "", errors.New("sendpulse belum dikonfigurasi")
	}

	tokenCache.Lock()
	defer tokenCache.Unlock()
	// Refresh two minutes early so an in-flight send never uses a token that
	// expires between this check and the request.
	if tokenCache.token != "" && time.Now().Before(tokenCache.expires.Add(-2*time.Minute)) {
		return tokenCache.token, nil
	}

	body, err := json.Marshal(map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     id,
		"client_secret": secret,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendpulseTokenEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		// OAuth failures answer with {"error":"invalid_client","error_description":"..."}
		// rather than the {"result":false} shape the send endpoint uses.
		var detail struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
			Message     string `json:"message"`
		}
		_ = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&detail)
		parts := []string{}
		for _, s := range []string{detail.Description, detail.Message, detail.Error} {
			if strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		if len(parts) == 0 {
			parts = append(parts, fmt.Sprintf("http %d", res.StatusCode))
		}
		return "", fmt.Errorf("sendpulse oauth: %s", strings.Join(parts, " "))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out); err != nil {
		return "", fmt.Errorf("sendpulse oauth response: %w", err)
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return "", errors.New("sendpulse oauth missing access_token")
	}
	ttl := out.ExpiresIn
	if ttl <= 0 {
		ttl = 3600
	}
	tokenCache.token = out.AccessToken
	tokenCache.expires = time.Now().Add(time.Duration(ttl) * time.Second)
	return tokenCache.token, nil
}

// emailDispatcherEnabled reports whether every value needed to actually send is
// present. When false the outbox is left untouched so queued mail survives until
// credentials are configured.
func emailDispatcherEnabled() bool {
	return sendpulseClientID() != "" && sendpulseClientSecret() != "" && emailSenderEmail() != ""
}

type emailQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type emailOutboxDB interface {
	execer
	emailQueryer
}

// enqueueEmailOutbox is called inside the payment transaction. The unique event
// key makes repeated provider webhooks unable to create duplicate emails.
func enqueueEmailOutbox(ctx context.Context, db emailOutboxDB, eventKey, eventType, userID, referenceID string) error {
	tag, err := db.Exec(ctx, `INSERT INTO email_outbox(event_key,event_type,user_id,reference_id)
		VALUES($1,$2,$3,$4) ON CONFLICT (event_key) DO NOTHING`, eventKey, eventType, userID, referenceID)
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	cleanupDraft := func() {
		_, _ = db.Exec(ctx, `DELETE FROM email_outbox WHERE event_key=$1 AND payload='{}'::jsonb`, eventKey)
	}
	payload, err := snapshotEmailPayload(ctx, db, emailOutboxRow{EventType: eventType, UserID: userID, ReferenceID: referenceID})
	if err != nil {
		cleanupDraft()
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		cleanupDraft()
		return err
	}
	_, err = db.Exec(ctx, `UPDATE email_outbox SET payload=$2::jsonb,updated_at=now()
		WHERE event_key=$1 AND payload='{}'::jsonb`, eventKey, string(data))
	if err != nil {
		cleanupDraft()
	}
	return err
}

type emailOutboxRow struct {
	ID          string
	EventKey    string
	EventType   string
	UserID      string
	ReferenceID string
	Payload     []byte
	Attempts    int
}

func claimEmailOutbox(ctx context.Context, db DB) (*emailOutboxRow, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Recover a worker interrupted after claiming a row. SendPulse requests have a
	// short timeout, so ten minutes is safely beyond a normal in-flight send.
	if _, err := tx.Exec(ctx, `UPDATE email_outbox SET status='FAILED', next_attempt_at=now(), updated_at=now()
		WHERE status='SENDING' AND updated_at < now() - interval '10 minutes'`); err != nil {
		return nil, err
	}

	var row emailOutboxRow
	err = tx.QueryRow(ctx, `SELECT id,event_key,event_type,user_id,reference_id,payload,attempts
		FROM email_outbox
		WHERE status IN ('PENDING','FAILED') AND next_attempt_at <= now() AND attempts < $1
		ORDER BY created_at
		LIMIT 1 FOR UPDATE SKIP LOCKED`, emailMaxAttempts).
		Scan(&row.ID, &row.EventKey, &row.EventType, &row.UserID, &row.ReferenceID, &row.Payload, &row.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE email_outbox SET status='SENDING', attempts=attempts+1, updated_at=now() WHERE id=$1`, row.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	row.Attempts++
	return &row, nil
}

type paymentEmail struct {
	To           string
	Locale       string
	Source       string
	CustomerName string
	Amount       float64
	Reference    string
	Environment  string
	Asset        string
	AssetAmount  string
	TxHash       string
	OrderTitle   string
	Quantity     int
	OccurredAt   time.Time
}

type emailTemplatePayload struct {
	To            string         `json:"to"`
	RecipientName string         `json:"recipient_name,omitempty"`
	Locale        string         `json:"locale"`
	Params        map[string]any `json:"params"`
}

func normalizeEmailLocale(locale string) string {
	if strings.EqualFold(strings.TrimSpace(locale), "en") {
		return "en"
	}
	return "id"
}

func loadPaymentEmail(ctx context.Context, db emailQueryer, row emailOutboxRow) (paymentEmail, error) {
	var out paymentEmail
	switch row.EventType {
	case emailEventDoku:
		err := db.QueryRow(ctx, `SELECT u.email,COALESCE(u.preferred_locale,'id'),COALESCE(u.name,''),c.amount,p.invoice_number,p.environment,now()
			FROM doku_payment p JOIN credit_transaction c ON c.id=p.credit_transaction_id
			JOIN "user" u ON u.id=p.user_id
			WHERE p.invoice_number=$1 AND p.user_id=$2 AND c.status='COMPLETED'`, row.ReferenceID, row.UserID).
			Scan(&out.To, &out.Locale, &out.CustomerName, &out.Amount, &out.Reference, &out.Environment, &out.OccurredAt)
		if err != nil {
			return out, err
		}
		out.Source = "DOKU Checkout"
	case emailEventManual:
		err := db.QueryRow(ctx, `SELECT u.email,COALESCE(u.preferred_locale,'id'),COALESCE(u.name,''),c.amount,c.id::text,now()
			FROM credit_transaction c JOIN "user" u ON u.id=c.user_id
			WHERE c.id=$1 AND c.user_id=$2 AND c.type='TOPUP' AND c.status='COMPLETED'`, row.ReferenceID, row.UserID).
			Scan(&out.To, &out.Locale, &out.CustomerName, &out.Amount, &out.Reference, &out.OccurredAt)
		if err != nil {
			return out, err
		}
		out.Source = "Top up manual"
	case emailEventUSDT:
		err := db.QueryRow(ctx, `SELECT u.email,COALESCE(u.preferred_locale,'id'),COALESCE(u.name,''),c.amount,p.id::text,p.expected_amount::text,COALESCE(p.tx_hash,''),COALESCE(p.paid_at,now())
 FROM crypto_payment p JOIN credit_transaction c ON c.id=p.credit_transaction_id
 JOIN "user" u ON u.id=p.user_id
 WHERE p.id=$1 AND p.user_id=$2 AND p.provider='TRON_TRC20' AND p.status='PAID' AND c.status='COMPLETED'`, row.ReferenceID, row.UserID).
			Scan(&out.To, &out.Locale, &out.CustomerName, &out.Amount, &out.Reference, &out.AssetAmount, &out.TxHash, &out.OccurredAt)
		if err != nil {
			return out, err
		}
		out.Source = "USDT TRC-20"
		out.Asset = "USDT"
	case emailEventMarket:
		err := db.QueryRow(ctx, `SELECT u.email,COALESCE(u.preferred_locale,'id'),COALESCE(u.name,''),o.subtotal,o.id::text,p.title,o.qty,o.created_at
			FROM market_order o JOIN market_product p ON p.id=o.product_id
			JOIN "user" u ON u.id=o.buyer_id
			WHERE o.id=$1 AND o.buyer_id=$2 AND o.status IN ('PAID','PROCESSING','COMPLETED')`, row.ReferenceID, row.UserID).
			Scan(&out.To, &out.Locale, &out.CustomerName, &out.Amount, &out.Reference, &out.OrderTitle, &out.Quantity, &out.OccurredAt)
		if err != nil {
			return out, err
		}
		out.Source = "Marketplace purchase"
	default:
		return out, fmt.Errorf("unknown email event type")
	}
	out.Locale = normalizeEmailLocale(out.Locale)
	return out, nil
}

func emailPublicURL(path string) string {
	base := strings.TrimRight(publicHTTPURL(os.Getenv("PUBLIC_WEB_URL")), "/")
	if base == "" {
		return ""
	}
	return base + path
}

func paymentEmailTemplatePayload(p paymentEmail) (emailTemplatePayload, error) {
	p.To = strings.TrimSpace(p.To)
	if p.To == "" {
		return emailTemplatePayload{}, errors.New("email recipient kosong")
	}
	locale := normalizeEmailLocale(p.Locale)
	amount := fmt.Sprintf("Rp %.0f", p.Amount)
	if p.Asset != "" {
		amount = fmt.Sprintf("%s · %s %s", amount, strings.TrimSpace(p.AssetAmount), p.Asset)
	}
	timestamp := ""
	if !p.OccurredAt.IsZero() {
		timestamp = p.OccurredAt.Format("02 Jan 2006 15:04 MST")
	}
	eventTitle := "Pembayaran berhasil"
	if locale == "en" {
		eventTitle = "Payment successful"
	}
	orderURL := ""
	if p.OrderTitle != "" {
		if locale == "en" {
			eventTitle = "Order successful"
		} else {
			eventTitle = "Pesanan berhasil"
		}
		orderURL = emailPublicURL("/pesanan/" + url.PathEscape(p.Reference) + "/chat")
	}
	return emailTemplatePayload{
		To:            p.To,
		RecipientName: strings.TrimSpace(p.CustomerName),
		Locale:        locale,
		Params: map[string]any{
			"event_title":   eventTitle,
			"customer_name": strings.TrimSpace(p.CustomerName),
			"method":        p.Source,
			"amount":        amount,
			"reference":     p.Reference,
			"timestamp":     timestamp,
			"dashboard_url": emailPublicURL("/dashboard"),
			"order_url":     orderURL,
			"telegram_url":  emailTelegramURL,
			"product_title": p.OrderTitle,
			"quantity":      p.Quantity,
		},
	}, nil
}

func snapshotEmailPayload(ctx context.Context, db emailQueryer, row emailOutboxRow) (emailTemplatePayload, error) {
	p, err := loadPaymentEmail(ctx, db, row)
	if err != nil {
		return emailTemplatePayload{}, err
	}
	return paymentEmailTemplatePayload(p)
}

func emailPayloadFromRow(ctx context.Context, db emailOutboxDB, row emailOutboxRow) (emailTemplatePayload, error) {
	raw := strings.TrimSpace(string(row.Payload))
	if raw != "" && raw != "{}" {
		var payload emailTemplatePayload
		if err := json.Unmarshal(row.Payload, &payload); err != nil {
			return emailTemplatePayload{}, fmt.Errorf("email payload: %w", err)
		}
		if strings.TrimSpace(payload.To) == "" || len(payload.Params) == 0 {
			return emailTemplatePayload{}, errors.New("email payload tidak lengkap")
		}
		payload.Locale = normalizeEmailLocale(payload.Locale)
		return payload, nil
	}

	// ponytail: legacy rows existed before payload snapshots; backfill once while claimed.
	payload, err := snapshotEmailPayload(ctx, db, row)
	if err != nil {
		return emailTemplatePayload{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return emailTemplatePayload{}, err
	}
	if _, err := db.Exec(ctx, `UPDATE email_outbox SET payload=$2::jsonb,updated_at=now()
		WHERE id=$1 AND status='SENDING' AND payload='{}'::jsonb`, row.ID, string(data)); err != nil {
		return emailTemplatePayload{}, err
	}
	return payload, nil
}

// sendEmail renders the stored payload locally and delivers it via SendPulse.
func sendEmail(ctx context.Context, eventKey string, p emailTemplatePayload) (string, error) {
	subject, htmlBody := renderEmail(p)
	return sendSendPulseEmail(ctx, subject, htmlBody, p)
}

func sendPasswordResetEmail(ctx context.Context, eventKey string, p emailTemplatePayload) (string, error) {
	subject, htmlBody := renderPasswordResetEmail(p)
	return sendSendPulseEmail(ctx, subject, htmlBody, p)
}

func sendSendPulseEmail(ctx context.Context, subject, htmlBody string, p emailTemplatePayload) (string, error) {
	if !emailDispatcherEnabled() {
		return "", errors.New("sendpulse belum dikonfigurasi")
	}
	if strings.TrimSpace(p.To) == "" || strings.TrimSpace(subject) == "" {
		return "", errors.New("payload email tidak lengkap")
	}

	to := map[string]string{"email": strings.TrimSpace(p.To)}
	if name := strings.TrimSpace(p.RecipientName); name != "" {
		to["name"] = name
	}
	// SendPulse wraps every message in an `email` object — sending the fields at
	// the top level makes the API answer "Argument email missing" (HTTP 422).
	// `html` is Base64-encoded and `auto_plain_text` asks SendPulse to derive the
	// plain-text part so clients without HTML still render something readable.
	payload, err := json.Marshal(map[string]any{
		"email": map[string]any{
			"from":            map[string]string{"email": emailSenderEmail(), "name": emailSenderName()},
			"to":              []map[string]string{to},
			"subject":         subject,
			"html":            base64.StdEncoding.EncodeToString([]byte(htmlBody)),
			"auto_plain_text": true,
		},
	})
	if err != nil {
		return "", err
	}

	token, err := sendpulseAccessToken(ctx)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendpulseEmailEndpoint, strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	// Success is {"result":true,"id":"<message-id>"}. Failures return a non-2xx
	// status and/or result:false with an error message.
	var response struct {
		Result  bool   `json:"result"`
		ID      string `json:"id"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	decodeErr := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&response)
	if res.StatusCode < 200 || res.StatusCode >= 300 || !response.Result {
		detail := strings.TrimSpace(strings.Join([]string{response.Message, response.Error}, " "))
		if detail == "" {
			detail = fmt.Sprintf("http %d", res.StatusCode)
		}
		return "", fmt.Errorf("sendpulse: %s", detail)
	}
	if decodeErr != nil {
		return "", fmt.Errorf("sendpulse response: %w", decodeErr)
	}
	if strings.TrimSpace(response.ID) == "" {
		return "", errors.New("sendpulse response missing id")
	}
	return response.ID, nil
}

func markEmailSent(ctx context.Context, db DB, id, providerMessageID string) error {
	_, err := db.Exec(ctx, `UPDATE email_outbox SET status='SENT',sent_at=now(),provider_message_id=$2,updated_at=now(),last_error=NULL WHERE id=$1 AND status='SENDING'`, id, providerMessageID)
	return err
}

func markEmailFailed(ctx context.Context, db DB, id string, attempts int, sendErr error) error {
	if attempts < 1 {
		attempts = 1
	}
	delay := time.Duration(1<<min(attempts, 6)) * time.Minute
	message := "email delivery failed"
	if sendErr != nil && strings.TrimSpace(sendErr.Error()) != "" {
		message = sendErr.Error()
	}
	if len(message) > 500 {
		message = message[:500]
	}
	_, err := db.Exec(ctx, `UPDATE email_outbox SET status='FAILED',next_attempt_at=now()+$2::interval,updated_at=now(),last_error=$3 WHERE id=$1 AND status='SENDING'`, id, fmt.Sprintf("%d minutes", int(delay/time.Minute)), message)
	return err
}

func dispatchEmailOutbox(ctx context.Context, db DB) bool {
	row, err := claimEmailOutbox(ctx, db)
	if err != nil {
		log.Printf("[WARN] email outbox claim failed: %v", err)
		return false
	}
	if row == nil {
		return false
	}
	p, err := emailPayloadFromRow(ctx, db, *row)
	var providerMessageID string
	if err == nil {
		providerMessageID, err = sendEmail(ctx, row.EventKey, p)
	}
	if err != nil {
		if markErr := markEmailFailed(ctx, db, row.ID, row.Attempts, err); markErr != nil {
			log.Printf("[WARN] email outbox failure update: %v", markErr)
		}
		return true
	}
	if err := markEmailSent(ctx, db, row.ID, providerMessageID); err != nil {
		log.Printf("[WARN] email outbox success update: %v", err)
	}
	return true
}

func StartEmailDispatcher(db DB, interval time.Duration) {
	if !emailDispatcherEnabled() {
		return
	}
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			for dispatchEmailOutbox(ctx, db) {
			}
			cancel()
			time.Sleep(interval)
		}
	}()
}

// ------------------------------------------------------------------ rendering

// esc escapes a value for both HTML text and attribute context. Every dynamic
// value in an email goes through this, so a customer name or product title can
// never inject markup into the message.
func esc(s string) string { return html.EscapeString(strings.TrimSpace(s)) }

func paramString(p emailTemplatePayload, key string) string {
	v, ok := p.Params[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

func emailParagraph(innerHTML string) string {
	return `<p style="margin:0 0 16px;font-size:15px;line-height:1.6;color:#9fb0c0;">` + innerHTML + `</p>`
}

func emailMuted(innerHTML string) string {
	return `<p style="margin:0 0 8px;font-size:13px;line-height:1.6;color:#7d8fa0;">` + innerHTML + `</p>`
}

func emailButton(href, label string) string {
	return `<table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 0 24px;">
        <tr><td style="background:#06b6d4;border-radius:9px;">
          <a href="` + esc(href) + `" style="display:inline-block;padding:13px 26px;font-size:15px;font-weight:600;color:#04131a;text-decoration:none;">` + esc(label) + `</a>
        </td></tr>
      </table>`
}

// emailShell wraps content in the shared Node2Mint chrome. The logo sits on a
// white plate because the PNG is a dark mark on a transparent field and would
// otherwise disappear against the dark background.
func emailShell(locale, heading, bodyHTML string) string {
	footer := "Butuh bantuan? <a href=\"" + esc(emailPublicURL("/contact")) + "\" style=\"color:#7dd3fc;\">Hubungi kami</a>.<br>Email ini dikirim otomatis, jangan dibalas."
	if locale == "en" {
		footer = "Need help? <a href=\"" + esc(emailPublicURL("/contact")) + "\" style=\"color:#7dd3fc;\">Contact us</a>.<br>This email is sent automatically, please do not reply."
	}
	return `<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;padding:0;background:#0b0f14;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#0b0f14;padding:32px 16px;">
    <tr><td align="center">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:520px;background:#111820;border:1px solid #1e2a36;border-radius:14px;">
        <tr><td style="padding:32px;">
          <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 0 24px;">
            <tr>
              <td valign="middle" style="padding-right:10px;">
                <div style="width:44px;height:44px;background:#ffffff;border-radius:10px;text-align:center;line-height:44px;">
                  <img src="` + esc(emailPublicURL("/node2mint-logo.png")) + `" width="34" height="34" alt="Node2Mint" style="display:inline-block;vertical-align:middle;border:0;">
                </div>
              </td>
              <td valign="middle" style="font-size:17px;font-weight:700;color:#e6edf3;">Node2Mint</td>
            </tr>
          </table>
          <h1 style="margin:0 0 16px;font-size:22px;line-height:1.3;color:#e6edf3;">` + esc(heading) + `</h1>
          ` + bodyHTML + `
          <hr style="border:none;border-top:1px solid #1e2a36;margin:24px 0 20px;">
          <p style="margin:0;font-size:12px;line-height:1.6;color:#5d6b78;">` + footer + `</p>
        </td></tr>
      </table>
      <p style="margin:20px 0 0;font-size:11px;color:#46525d;">© Node2Mint</p>
    </td></tr>
  </table>
</body></html>`
}

func renderPasswordResetEmail(p emailTemplatePayload) (string, string) {
	locale := normalizeEmailLocale(p.Locale)
	resetURL := paramString(p, "reset_url")
	name := paramString(p, "customer_name")
	minutes := paramString(p, "expires_minutes")
	if minutes == "" {
		minutes = "15"
	}

	if locale == "en" {
		greeting := "Hi"
		if name != "" {
			greeting = "Hi " + name + ","
		}
		body := emailParagraph(esc(greeting) + " we received a request to reset the password for your account.")
		body += emailButton(resetURL, "Set a new password")
		body += emailMuted("This link expires in " + esc(minutes) + " minutes and can only be used once.")
		body += emailMuted("If you did not request this, you can safely ignore this email — your password has not changed.")
		return "Reset your Node2Mint password", emailShell(locale, "Reset your password", body)
	}

	greeting := "Hai"
	if name != "" {
		greeting = "Hai " + name + ","
	}
	body := emailParagraph(esc(greeting) + " kami menerima permintaan untuk mengatur ulang kata sandi akunmu.")
	body += emailButton(resetURL, "Tetapkan kata sandi baru")
	body += emailMuted("Tautan ini kedaluwarsa dalam " + esc(minutes) + " menit dan hanya bisa dipakai sekali.")
	body += emailMuted("Kalau kamu tidak meminta ini, abaikan saja email ini — kata sandimu tidak berubah.")
	return "Atur ulang kata sandi Node2Mint", emailShell(locale, "Atur ulang kata sandi", body)
}

func renderEmail(p emailTemplatePayload) (string, string) {
	locale := normalizeEmailLocale(p.Locale)
	eventTitle := paramString(p, "event_title")
	name := paramString(p, "customer_name")
	method := paramString(p, "method")
	amount := paramString(p, "amount")
	reference := paramString(p, "reference")
	timestamp := paramString(p, "timestamp")
	orderURL := paramString(p, "order_url")
	productTitle := paramString(p, "product_title")
	quantity := paramString(p, "quantity")

	greeting := "Hai"
	successLabel := "Pembayaran kamu sudah kami terima."
	if locale == "en" {
		greeting = "Hi"
		successLabel = "We have received your payment."
	}
	if name != "" {
		greeting += " " + name
	}
	greeting += ","
	body := emailParagraph(esc(greeting) + " " + esc(successLabel))

	// Only non-empty values are rendered, so an order email does not show blank
	// payment fields and a top-up email does not show an empty product row.
	type row struct{ label, value string }
	rows := []row{}
	add := func(labelID, labelEN, value string) {
		if value == "" || value == "0" {
			return
		}
		label := labelID
		if locale == "en" {
			label = labelEN
		}
		rows = append(rows, row{label, value})
	}
	add("Metode", "Method", method)
	add("Jumlah", "Amount", amount)
	add("Produk", "Product", productTitle)
	add("Jumlah item", "Quantity", quantity)
	add("Referensi", "Reference", reference)
	add("Waktu", "Time", timestamp)

	if len(rows) > 0 {
		body += `<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;border:1px solid #1e2a36;border-radius:10px;">`
		for i, r := range rows {
			border := ""
			if i > 0 {
				border = "border-top:1px solid #1e2a36;"
			}
			body += `<tr>
              <td style="padding:10px 14px;font-size:13px;color:#7d8fa0;` + border + `">` + esc(r.label) + `</td>
              <td style="padding:10px 14px;font-size:13px;color:#e6edf3;text-align:right;` + border + `">` + esc(r.value) + `</td>
            </tr>`
		}
		body += `</table>`
	}

	if orderURL != "" {
		label := "Lihat pesanan"
		if locale == "en" {
			label = "View order"
		}
		body += emailButton(orderURL, label)
	} else if dashboard := emailPublicURL("/dashboard"); dashboard != "" {
		label := "Buka dasbor"
		if locale == "en" {
			label = "Open dashboard"
		}
		body += emailButton(dashboard, label)
	}

	heading := eventTitle
	if heading == "" {
		heading = "Konfirmasi pembayaran Node2Mint"
		if locale == "en" {
			heading = "Node2Mint payment confirmation"
		}
	}
	return heading, emailShell(locale, heading, body)
}
