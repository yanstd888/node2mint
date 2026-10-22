package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTelegramGateWebhookSecretAndStates(t *testing.T) {
	if !telegramActiveStatus("member") || !telegramActiveStatus("administrator") || telegramActiveStatus("left") {
		t.Fatal("telegram membership status mapping changed")
	}
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "test-secret")
	h := TelegramWebhook(nil)
	bad := httptest.NewRecorder()
	h.ServeHTTP(bad, httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", strings.NewReader(`{}`)))
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("missing webhook secret status=%d", bad.Code)
	}
	goodReq := httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", strings.NewReader(`{}`))
	goodReq.Header.Set("X-Telegram-Bot-Api-Secret-Token", "test-secret")
	good := httptest.NewRecorder()
	h.ServeHTTP(good, goodReq)
	if good.Code != http.StatusOK {
		t.Fatalf("valid webhook secret status=%d", good.Code)
	}
	locked := httptest.NewRecorder()
	telegramGateWriteError(locked, telegramLockedNeverVerified, http.StatusForbidden)
	if locked.Code != http.StatusForbidden || !strings.Contains(locked.Body.String(), "TELEGRAM_MEMBERSHIP_REQUIRED") || !strings.Contains(locked.Body.String(), "LOCKED_NEVER_VERIFIED") {
		t.Fatalf("gate response=%d %q", locked.Code, locked.Body.String())
	}
}
