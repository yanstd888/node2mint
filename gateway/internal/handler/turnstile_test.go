package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyTurnstile(t *testing.T) {
	oldURL := turnstileSiteVerifyURL
	defer func() { turnstileSiteVerifyURL = oldURL }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Form.Get("secret") != "secret" || r.Form.Get("response") != "token" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"hostname":"www.node2mint.com","action":"auth"}`))
	}))
	defer server.Close()
	turnstileSiteVerifyURL = server.URL
	t.Setenv("TURNSTILE_SITE_KEY", "site")
	t.Setenv("TURNSTILE_SECRET_KEY", "secret")
	t.Setenv("TURNSTILE_EXPECTED_HOSTNAME", "www.node2mint.com")

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	if err := verifyTurnstile(req.Context(), req, "token", turnstileActionAuth); err != nil {
		t.Fatalf("valid Turnstile response rejected: %v", err)
	}
	if err := verifyTurnstile(req.Context(), req, "", turnstileActionAuth); err == nil {
		t.Fatal("missing Turnstile token must be rejected when configured")
	}
}
