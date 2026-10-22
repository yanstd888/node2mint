package handler

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestValidateOAuthConfigProductionRequiresHTTPSAndCompletePairs(t *testing.T) {
	t.Setenv("NODE_ENV", "production")
	t.Setenv("PUBLIC_WEB_URL", "http://example.test")
	t.Setenv("PUBLIC_API_URL", "https://example.test")
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	t.Setenv("TELEGRAM_CLIENT_ID", "")
	t.Setenv("TELEGRAM_CLIENT_SECRET", "")
	if err := ValidateOAuthConfig(); err == nil {
		t.Fatal("production HTTP web URL must be rejected")
	}

	t.Setenv("PUBLIC_WEB_URL", "https://example.test")
	t.Setenv("GOOGLE_CLIENT_ID", "configured")
	if err := ValidateOAuthConfig(); err == nil {
		t.Fatal("partial Google credentials must be rejected")
	}
}

func TestOAuthPKCECookie(t *testing.T) {
	t.Setenv("PUBLIC_API_URL", "https://example.test")
	rr := httptest.NewRecorder()
	verifier, challenge, err := oauthPKCE(rr, "ga_cv")
	if err != nil {
		t.Fatalf("oauthPKCE() error = %v", err)
	}
	sum := sha256.Sum256([]byte(verifier))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); challenge != want {
		t.Fatalf("challenge=%q, want %q", challenge, want)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "ga_cv" || cookies[0].Value != verifier {
		t.Fatalf("unexpected PKCE cookie: %#v", cookies)
	}
	if !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].MaxAge != 600 {
		t.Fatalf("unsafe PKCE cookie attributes: %#v", cookies[0])
	}
}

func TestGoogleStartUsesPKCEAndRejectsDisabledProvider(t *testing.T) {
	t.Setenv("PUBLIC_API_URL", "https://example.test")
	t.Setenv("TURNSTILE_SITE_KEY", "")
	t.Setenv("TURNSTILE_SECRET_KEY", "")
	t.Setenv("GOOGLE_CLIENT_ID", "configured")
	t.Setenv("GOOGLE_CLIENT_SECRET", "configured")
	rr := httptest.NewRecorder()
	GoogleStart(rr, httptest.NewRequest(http.MethodGet, "/api/auth/google", nil))
	if rr.Code != http.StatusFound {
		t.Fatalf("GoogleStart status=%d, want %d", rr.Code, http.StatusFound)
	}
	loc, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := loc.Query()
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("GoogleStart missing PKCE: %s", loc.RawQuery)
	}

	t.Setenv("GOOGLE_CLIENT_ID", "")
	rr = httptest.NewRecorder()
	GoogleStart(rr, httptest.NewRequest(http.MethodGet, "/api/auth/google", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("disabled GoogleStart status=%d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestOAuthStartRequiresTurnstileWhenConfigured(t *testing.T) {
	t.Setenv("TURNSTILE_SITE_KEY", "site")
	t.Setenv("TURNSTILE_SECRET_KEY", "secret")
	t.Setenv("GOOGLE_CLIENT_ID", "configured")
	t.Setenv("GOOGLE_CLIENT_SECRET", "configured")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/start", strings.NewReader(`{"provider":"google"}`))
	req.Header.Set("Content-Type", "application/json")
	OAuthStart(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("OAuthStart status=%d, want %d", rr.Code, http.StatusForbidden)
	}
}
