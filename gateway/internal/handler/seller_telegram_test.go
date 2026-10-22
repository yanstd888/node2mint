package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSellerTelegramMessageEscapesDynamicFields(t *testing.T) {
	text, label, buttonURL := sellerTelegramMessage("NEW_ORDER", sellerTelegramPayload{
		Store: "Toko <utama>", Title: "Produk & aman", Quantity: 2, Amount: 50000, OrderID: "order-1",
	})
	if strings.Contains(text, "Toko <utama>") || !strings.Contains(text, "Toko &lt;utama&gt;") || !strings.Contains(text, "Produk &amp; aman") {
		t.Fatalf("dynamic fields were not escaped: %q", text)
	}
	if label != "" || buttonURL != "" {
		t.Fatalf("button should be empty without an HTTPS public URL: label=%q url=%q", label, buttonURL)
	}
}

func TestSellerTelegramMessageHasRequiredEventContracts(t *testing.T) {
	t.Setenv("PUBLIC_WEB_URL", "https://www.node2mint.com")
	payload := sellerTelegramPayload{
		Store: "Toko Aman", Title: "Produk Aman", Quantity: 2, Amount: 50000,
		Stock: 2, Threshold: 3, OrderID: "order-1", ProductID: "product-1",
		DisputeID: "dispute-1", Reason: "Barang tidak sesuai deskripsi", OccurredAt: "08 Okt 2026 12:34 WIB", ResponseDeadline: "09 Okt 2026 12:34 WIB",
	}
	cases := []struct {
		event, wantLabel, wantURL string
		want                      []string
	}{
		{"NEW_ORDER", "Buka Pesanan", "/toko?tab=pesanan", []string{"Order: <code>order-1</code>", "Waktu: 08 Okt 2026 12:34 WIB", "Rp 50000", "1×24 jam"}},
		{"DISPUTE", "Buka Dispute", "/toko/disputes", []string{"Order: <code>order-1</code>", "Waktu: 08 Okt 2026 12:34 WIB", "Tenggat respons: <b>09 Okt 2026 12:34 WIB</b>", "Jenis: Sengketa pembeli", "Barang tidak sesuai deskripsi"}},
		{"LOW_STOCK", "Kelola Produk", "/toko?tab=produk", []string{"ID Produk: <code>product-1</code>", "Waktu: 08 Okt 2026 12:34 WIB", "Nilai order: <b>Rp 50000</b>"}},
		{"OUT_OF_STOCK", "Kelola Produk", "/toko?tab=produk", []string{"ID Produk: <code>product-1</code>", "Waktu: 08 Okt 2026 12:34 WIB", "Nilai order: <b>Rp 50000</b>"}},
	}
	for _, tc := range cases {
		t.Run(tc.event, func(t *testing.T) {
			text, label, buttonURL := sellerTelegramMessage(tc.event, payload)
			if label != tc.wantLabel || !strings.Contains(buttonURL, tc.wantURL) {
				t.Fatalf("button label/url = %q %q", label, buttonURL)
			}
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Fatalf("missing %q in %q", want, text)
				}
			}
		})
	}
}

func TestSellerTelegramTimeUsesWIB(t *testing.T) {
	got := sellerTelegramTime(time.Date(2026, time.October, 8, 5, 34, 0, 0, time.UTC))
	if got != "08 Okt 2026 12:34 WIB" {
		t.Fatalf("sellerTelegramTime() = %q", got)
	}
}

func TestEnqueueSellerTelegramAddsOccurredAt(t *testing.T) {
	capture := &captureExecer{}
	if err := enqueueSellerTelegram(context.Background(), capture, "event-1", "NEW_ORDER", "seller-1", "order-1", "product-1", sellerTelegramPayload{}); err != nil {
		t.Fatalf("enqueueSellerTelegram() error = %v", err)
	}
	if len(capture.args) != 1 {
		t.Fatalf("enqueue calls = %d, want 1", len(capture.args))
	}
	raw, _ := capture.args[0][5].(string)
	var payload sellerTelegramPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("payload JSON = %q: %v", raw, err)
	}
	if payload.OccurredAt == "" {
		t.Fatal("enqueued seller Telegram payload must record occurred_at")
	}
}

func TestSendTelegramSellerMessageUsesInlineButton(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		text := string(body)
		if !strings.Contains(text, "chat_id=123") || !strings.Contains(text, "reply_markup=") || !strings.Contains(text, "Kelola+Produk") {
			t.Errorf("unexpected Telegram form: %s", text)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	oldBase := telegramBotAPIBase
	telegramBotAPIBase = server.URL
	t.Cleanup(func() { telegramBotAPIBase = oldBase })
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")

	if err := sendTelegramSellerMessage(context.Background(), 123, "<b>test</b>", "Kelola Produk", "https://example.test/toko"); err != nil {
		t.Fatalf("sendTelegramSellerMessage() error = %v", err)
	}
}
