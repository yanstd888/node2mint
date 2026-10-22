package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type emailSnapshotTestRow struct {
	values []any
	err    error
}

// keysOf returns the sorted top-level keys of a decoded JSON object, so a
// failing assertion can show what was actually sent.
func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (r emailSnapshotTestRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return io.ErrUnexpectedEOF
	}
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *string:
			*target = value.(string)
		case *float64:
			*target = value.(float64)
		case *int:
			*target = value.(int)
		case *time.Time:
			*target = value.(time.Time)
		default:
			return io.ErrUnexpectedEOF
		}
	}
	return nil
}

type emailOutboxCapture struct {
	row        emailSnapshotTestRow
	insertTag  pgconn.CommandTag
	queryCount int
	execSQL    []string
	execArgs   [][]any
}

func (c *emailOutboxCapture) QueryRow(context.Context, string, ...any) pgx.Row {
	c.queryCount++
	return c.row
}

func (c *emailOutboxCapture) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	c.execSQL = append(c.execSQL, sql)
	c.execArgs = append(c.execArgs, append([]any(nil), args...))
	if len(c.execSQL) == 1 {
		return c.insertTag, nil
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func TestEnqueueEmailOutboxStoresTemplateSnapshot(t *testing.T) {
	t.Setenv("PUBLIC_WEB_URL", "https://www.node2mint.com")
	db := &emailOutboxCapture{
		insertTag: pgconn.NewCommandTag("INSERT 0 1"),
		row: emailSnapshotTestRow{values: []any{
			"buyer@example.test", "en", "Buyer", float64(50000), "order-1", "Premium Prompt", 2,
			time.Date(2026, time.October, 6, 12, 34, 0, 0, time.FixedZone("WIB", 7*60*60)),
		}},
	}
	if err := enqueueEmailOutbox(context.Background(), db, "market:order-1", emailEventMarket, "buyer-1", "order-1"); err != nil {
		t.Fatalf("enqueueEmailOutbox() error = %v", err)
	}
	if len(db.execSQL) != 2 || !strings.Contains(db.execSQL[1], "SET payload") {
		t.Fatalf("outbox statements = %#v", db.execSQL)
	}
	raw, _ := db.execArgs[1][1].(string)
	var payload emailTemplatePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.To != "buyer@example.test" || payload.Params["reference"] != "order-1" || payload.Params["telegram_url"] != emailTelegramURL {
		t.Fatalf("snapshot = %#v", payload)
	}
}

func TestEnqueueEmailOutboxSkipsSnapshotForDuplicate(t *testing.T) {
	db := &emailOutboxCapture{insertTag: pgconn.NewCommandTag("INSERT 0 0")}
	if err := enqueueEmailOutbox(context.Background(), db, "market:order-1", emailEventMarket, "buyer-1", "order-1"); err != nil {
		t.Fatalf("enqueueEmailOutbox() error = %v", err)
	}
	if db.queryCount != 0 || len(db.execSQL) != 1 {
		t.Fatalf("duplicate must not rebuild snapshot: queries=%d statements=%d", db.queryCount, len(db.execSQL))
	}
}

func TestEnqueueEmailOutboxRemovesDraftWhenSnapshotFails(t *testing.T) {
	db := &emailOutboxCapture{
		insertTag: pgconn.NewCommandTag("INSERT 0 1"),
		row:       emailSnapshotTestRow{err: io.ErrUnexpectedEOF},
	}
	if err := enqueueEmailOutbox(context.Background(), db, "market:order-1", emailEventMarket, "buyer-1", "order-1"); err == nil {
		t.Fatal("snapshot error must be returned")
	}
	if len(db.execSQL) != 2 || !strings.Contains(db.execSQL[1], "DELETE FROM email_outbox") {
		t.Fatalf("failed snapshot must remove draft: %#v", db.execSQL)
	}
}

func TestPaymentEmailTemplatePayloadIncludesRequiredParams(t *testing.T) {
	t.Setenv("PUBLIC_WEB_URL", "https://www.node2mint.com")
	payload, err := paymentEmailTemplatePayload(paymentEmail{
		To: "buyer@example.test", Locale: "en", Source: "Marketplace purchase", Amount: 50000,
		Reference: "order-1", CustomerName: "Buyer", OrderTitle: "Premium Prompt", Quantity: 2,
		OccurredAt: time.Date(2026, time.October, 6, 12, 34, 0, 0, time.FixedZone("WIB", 7*60*60)),
	})
	if err != nil {
		t.Fatalf("paymentEmailTemplatePayload() error = %v", err)
	}
	if payload.To != "buyer@example.test" || payload.Locale != "en" {
		t.Fatalf("recipient/locale = %#v", payload)
	}
	for key, want := range map[string]string{
		"event_title":   "Order successful",
		"method":        "Marketplace purchase",
		"amount":        "Rp 50000",
		"reference":     "order-1",
		"timestamp":     "06 Oct 2026 12:34 WIB",
		"dashboard_url": "https://www.node2mint.com/dashboard",
		"order_url":     "https://www.node2mint.com/pesanan/order-1/chat",
		"telegram_url":  "https://t.me/node2mint",
		"product_title": "Premium Prompt",
	} {
		if got, _ := payload.Params[key].(string); got != want {
			t.Fatalf("params[%q] = %q, want %q", key, got, want)
		}
	}
	if got, _ := payload.Params["quantity"].(int); got != 2 {
		t.Fatalf("quantity = %d, want 2", got)
	}
}

func TestEmailDispatcherRequiresCredentialsAndSender(t *testing.T) {
	t.Setenv("SENDPULSE_CLIENT_ID", "test-id")
	t.Setenv("SENDPULSE_CLIENT_SECRET", "test-secret")
	t.Setenv("EMAIL_SENDER_EMAIL", "noreply@example.test")
	if !emailDispatcherEnabled() {
		t.Fatal("dispatcher must start when credentials and sender are configured")
	}
	// A missing sender must hold queued mail rather than attempt a send.
	t.Setenv("EMAIL_SENDER_EMAIL", "")
	if emailDispatcherEnabled() {
		t.Fatal("dispatcher must leave queued emails untouched without a sender address")
	}
	t.Setenv("EMAIL_SENDER_EMAIL", "noreply@example.test")
	t.Setenv("SENDPULSE_CLIENT_SECRET", "")
	if emailDispatcherEnabled() {
		t.Fatal("dispatcher must not run without a client secret")
	}
}

func TestEmailOutboxSchemaStoresPayload(t *testing.T) {
	schema, err := os.ReadFile("../../sql/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"payload        JSONB NOT NULL DEFAULT '{}'::jsonb",
		"ALTER TABLE email_outbox ADD COLUMN IF NOT EXISTS payload JSONB NOT NULL DEFAULT '{}'::jsonb",
	} {
		if !strings.Contains(string(schema), want) {
			t.Fatalf("schema.sql missing %q", want)
		}
	}
}

func TestEmailRetryContract(t *testing.T) {
	if emailMaxAttempts != 3 {
		t.Fatalf("emailMaxAttempts=%d, want 3 total attempts", emailMaxAttempts)
	}
}

func TestSendEmailPostsBase64HTMLToSendPulse(t *testing.T) {
	var sawEmail bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]json.RawMessage
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("invalid JSON: %v", err)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization=%q", got)
		}
		// SendPulse requires the whole message nested under `email`; top-level
		// fields are rejected with HTTP 422 "Argument email missing".
		inner, ok := payload["email"]
		if !ok {
			t.Fatalf("payload must nest the message under `email`, got keys: %v", keysOf(payload))
		}
		var msg map[string]json.RawMessage
		if err := json.Unmarshal(inner, &msg); err != nil {
			t.Fatalf("email object invalid: %v", err)
		}
		var subject string
		_ = json.Unmarshal(msg["subject"], &subject)
		if subject == "" {
			t.Error("subject must be set for an inline-HTML send")
		}
		// The HTML body must arrive Base64-encoded, not as raw markup.
		var encoded string
		if err := json.Unmarshal(msg["html"], &encoded); err != nil {
			t.Fatalf("html field missing inside email object: %v", err)
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("html must be Base64: %v", err)
		}
		if !strings.Contains(string(decoded), "N2M-1") {
			t.Errorf("rendered HTML must include the reference, got %d bytes", len(decoded))
		}
		if _, hasTemplate := msg["template"]; hasTemplate {
			t.Error("inline-HTML send must not send a template")
		}
		// The sender must arrive under `from`.
		var from map[string]string
		if err := json.Unmarshal(msg["from"], &from); err != nil || from["email"] != "noreply@example.test" {
			t.Errorf("from=%v err=%v", from, err)
		}
		var to []map[string]string
		if err := json.Unmarshal(msg["to"], &to); err != nil || len(to) != 1 || to[0]["email"] != "buyer@example.test" {
			t.Errorf("to=%v err=%v", to, err)
		}
		sawEmail = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":true,"id":"<sendpulse-1@example.test>"}`))
	}))
	defer server.Close()

	oldEmail, oldToken := sendpulseEmailEndpoint, sendpulseTokenEndpoint
	sendpulseEmailEndpoint = server.URL
	sendpulseTokenEndpoint = server.URL
	t.Cleanup(func() { sendpulseEmailEndpoint, sendpulseTokenEndpoint = oldEmail, oldToken })

	t.Setenv("SENDPULSE_CLIENT_ID", "test-id")
	t.Setenv("SENDPULSE_CLIENT_SECRET", "test-secret")
	t.Setenv("EMAIL_SENDER_EMAIL", "noreply@example.test")
	t.Setenv("PUBLIC_WEB_URL", "https://www.node2mint.com")

	// Prime the token cache so the stub only has to answer the send request.
	tokenCache.Lock()
	tokenCache.token = "test-token"
	tokenCache.expires = time.Now().Add(time.Hour)
	tokenCache.Unlock()
	t.Cleanup(func() {
		tokenCache.Lock()
		tokenCache.token, tokenCache.expires = "", time.Time{}
		tokenCache.Unlock()
	})

	messageID, err := sendEmail(context.Background(), "doku:N2M-1", emailTemplatePayload{
		To: "buyer@example.test", Locale: "en", Params: map[string]any{
			"event_title": "Payment successful", "reference": "N2M-1",
		},
	})
	if err != nil {
		t.Fatalf("sendEmail() error = %v", err)
	}
	if !sawEmail {
		t.Fatal("send request never reached the endpoint")
	}
	if messageID != "<sendpulse-1@example.test>" {
		t.Fatalf("messageID=%q", messageID)
	}
}

func TestSendPulseFailureIsReported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"result":false,"error":"invalid token"}`))
	}))
	defer server.Close()

	oldEmail := sendpulseEmailEndpoint
	sendpulseEmailEndpoint = server.URL
	t.Cleanup(func() { sendpulseEmailEndpoint = oldEmail })

	t.Setenv("SENDPULSE_CLIENT_ID", "test-id")
	t.Setenv("SENDPULSE_CLIENT_SECRET", "test-secret")
	t.Setenv("EMAIL_SENDER_EMAIL", "noreply@example.test")

	tokenCache.Lock()
	tokenCache.token = "test-token"
	tokenCache.expires = time.Now().Add(time.Hour)
	tokenCache.Unlock()
	t.Cleanup(func() {
		tokenCache.Lock()
		tokenCache.token, tokenCache.expires = "", time.Time{}
		tokenCache.Unlock()
	})

	_, err := sendEmail(context.Background(), "k", emailTemplatePayload{
		To: "buyer@example.test", Locale: "id", Params: map[string]any{"reference": "X"},
	})
	if err == nil {
		t.Fatal("a non-2xx SendPulse response must surface as an error so the outbox retries")
	}
	if !strings.Contains(err.Error(), "invalid token") {
		t.Fatalf("error must carry the provider detail, got %v", err)
	}
}

func TestPasswordResetEmailRendersItsOwnSubjectAndLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]json.RawMessage
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("invalid JSON: %v", err)
		}
		var msg map[string]json.RawMessage
		if inner, ok := payload["email"]; !ok {
			t.Fatalf("payload must nest the message under `email`, got keys: %v", keysOf(payload))
		} else if err := json.Unmarshal(inner, &msg); err != nil {
			t.Fatalf("email object invalid: %v", err)
		}
		var subject string
		_ = json.Unmarshal(msg["subject"], &subject)
		if subject != "Reset your Node2Mint password" {
			t.Errorf("subject=%q", subject)
		}
		var encoded string
		if err := json.Unmarshal(msg["html"], &encoded); err != nil {
			t.Fatalf("html field missing: %v", err)
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("html must be Base64: %v", err)
		}
		htmlBody := string(decoded)
		// The reset link and the one-time-use warning must both be present, and
		// the token must appear only inside the link the user clicks.
		if !strings.Contains(htmlBody, "https://www.node2mint.com/reset-password?token=token-value") {
			t.Error("reset_url must be rendered into the HTML")
		}
		if !strings.Contains(htmlBody, "only be used once") {
			t.Error("English template must state the link is single-use")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":true,"id":"<sendpulse-reset-1@example.test>"}`))
	}))
	defer server.Close()

	oldEmail, oldToken := sendpulseEmailEndpoint, sendpulseTokenEndpoint
	sendpulseEmailEndpoint = server.URL
	sendpulseTokenEndpoint = server.URL
	t.Cleanup(func() { sendpulseEmailEndpoint, sendpulseTokenEndpoint = oldEmail, oldToken })

	t.Setenv("SENDPULSE_CLIENT_ID", "test-id")
	t.Setenv("SENDPULSE_CLIENT_SECRET", "test-secret")
	t.Setenv("EMAIL_SENDER_EMAIL", "noreply@example.test")
	t.Setenv("PUBLIC_WEB_URL", "https://www.node2mint.com")

	tokenCache.Lock()
	tokenCache.token = "test-token"
	tokenCache.expires = time.Now().Add(time.Hour)
	tokenCache.Unlock()
	t.Cleanup(func() {
		tokenCache.Lock()
		tokenCache.token, tokenCache.expires = "", time.Time{}
		tokenCache.Unlock()
	})

	messageID, err := sendPasswordResetEmail(context.Background(), "reset:user-1", emailTemplatePayload{
		To: "owner@example.test", Locale: "en", Params: map[string]any{
			"reset_url":     "https://www.node2mint.com/reset-password?token=token-value",
			"customer_name": "Owner", "expires_minutes": "15",
		},
	})
	if err != nil {
		t.Fatalf("sendPasswordResetEmail() error = %v", err)
	}
	if messageID != "<sendpulse-reset-1@example.test>" {
		t.Fatalf("messageID=%q", messageID)
	}
}

func TestRenderEscapesCustomerSuppliedValues(t *testing.T) {
	_, htmlBody := renderEmail(emailTemplatePayload{
		To: "buyer@example.test", Locale: "id",
		Params: map[string]any{
			"customer_name": `<script>alert(1)</script>`,
			"product_title": `"><img src=x onerror=alert(1)>`,
			"reference":     "N2M-1",
		},
	})
	// Escaped text may still mention "onerror" as literal characters; what must
	// never appear is an executable tag or an unquoted attribute break-out.
	if strings.Contains(htmlBody, "<script>") {
		t.Fatal("script tag must never be injected as markup")
	}
	if strings.Contains(htmlBody, "<img src=x") {
		t.Fatal("img tag must never be injected as markup")
	}
	if !strings.Contains(htmlBody, "&lt;script&gt;") {
		t.Fatal("escaped value should still be visible to the reader as text")
	}
	// The attribute-breaking quote must be escaped, so the payload cannot close
	// out of the td and open its own attribute.
	if strings.Contains(htmlBody, `style="color:#e6edf3;">"><img`) {
		t.Fatal("a quote in a customer value must be escaped, not break out of the attribute")
	}
}
