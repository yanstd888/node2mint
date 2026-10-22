package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const turnstileActionAuth = "auth"

func turnstileSiteKey() string {
	if !turnstileEnabled() {
		return ""
	}
	return strings.TrimSpace(os.Getenv("TURNSTILE_SITE_KEY"))
}

func turnstileEnabled() bool {
	return strings.TrimSpace(os.Getenv("TURNSTILE_SITE_KEY")) != "" && strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY")) != ""
}

// ValidateTurnstileConfig rejects partial production configuration before any
// login endpoint can be locked behind an invisible, unrendered widget.
func ValidateTurnstileConfig() error {
	siteKey := strings.TrimSpace(os.Getenv("TURNSTILE_SITE_KEY"))
	secret := strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY"))
	if (siteKey == "") != (secret == "") {
		return errors.New("TURNSTILE_SITE_KEY dan TURNSTILE_SECRET_KEY harus diisi bersama")
	}
	if turnstileEnabled() && strings.HasPrefix(webURL(), "https://") && strings.TrimSpace(os.Getenv("TURNSTILE_EXPECTED_HOSTNAME")) == "" {
		return errors.New("TURNSTILE_EXPECTED_HOSTNAME wajib untuk Turnstile produksi")
	}
	return nil
}

var turnstileSiteVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var turnstileClient = &http.Client{Timeout: 5 * time.Second}

// verifyTurnstile is fail-open only while no Turnstile secret is configured.
// Once configured, a fresh, single-use token is mandatory before auth actions.
func verifyTurnstile(ctx context.Context, r *http.Request, token, action string) error {
	secret := strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY"))
	if !turnstileEnabled() {
		return nil
	}
	if len(token) == 0 || len(token) > 2048 {
		return errors.New("verifikasi keamanan diperlukan")
	}

	form := url.Values{
		"secret":   {secret},
		"response": {token},
	}
	if ip := ipOf(r); ip != "" {
		form.Set("remoteip", ip)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileSiteVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return errors.New("verifikasi keamanan gagal")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := turnstileClient.Do(req)
	if err != nil {
		return errors.New("verifikasi keamanan gagal")
	}
	defer res.Body.Close()
	var out struct {
		Success  bool   `json:"success"`
		Hostname string `json:"hostname"`
		Action   string `json:"action"`
	}
	if res.StatusCode != http.StatusOK || json.NewDecoder(res.Body).Decode(&out) != nil || !out.Success {
		return errors.New("verifikasi keamanan gagal")
	}
	if action != "" && out.Action != action {
		return errors.New("verifikasi keamanan gagal")
	}
	if host := strings.TrimSpace(os.Getenv("TURNSTILE_EXPECTED_HOSTNAME")); host != "" && !strings.EqualFold(out.Hostname, host) {
		return errors.New("verifikasi keamanan gagal")
	}
	return nil
}
