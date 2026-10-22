package handler

// Login sosial: Google dan Telegram authorization-code/OIDC. Semua provider
// butuh env; yang tanpa env dilaporkan disabled oleh /api/auth/providers.
// Sukses = issue JWT yang sama dengan Login() + HttpOnly session cookie +
// redirect bersih ke /auth/callback.
import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"ai-sell/gateway/internal/auth"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func webURL() string {
	if u := strings.TrimRight(os.Getenv("PUBLIC_WEB_URL"), "/"); u != "" {
		return u
	}
	return "http://localhost:5000"
}

func apiURL() string {
	if u := strings.TrimRight(os.Getenv("PUBLIC_API_URL"), "/"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

func googleOAuthEnabled() bool {
	return os.Getenv("GOOGLE_CLIENT_ID") != "" && os.Getenv("GOOGLE_CLIENT_SECRET") != ""
}

// ValidateOAuthConfig fails closed for production redirect/cookie settings and
// rejects half-configured provider pairs before the server starts.
func ValidateOAuthConfig() error {
	if strings.EqualFold(os.Getenv("NODE_ENV"), "production") {
		if !strings.HasPrefix(webURL(), "https://") || !strings.HasPrefix(apiURL(), "https://") {
			return fmt.Errorf("PUBLIC_WEB_URL dan PUBLIC_API_URL wajib HTTPS di production")
		}
	}
	if (os.Getenv("GOOGLE_CLIENT_ID") == "") != (os.Getenv("GOOGLE_CLIENT_SECRET") == "") {
		return fmt.Errorf("GOOGLE_CLIENT_ID dan GOOGLE_CLIENT_SECRET harus diisi bersama")
	}
	if (os.Getenv("TELEGRAM_CLIENT_ID") == "") != (os.Getenv("TELEGRAM_CLIENT_SECRET") == "") {
		return fmt.Errorf("TELEGRAM_CLIENT_ID dan TELEGRAM_CLIENT_SECRET harus diisi bersama")
	}
	return nil
}

// AuthProviders: GET /api/auth/providers (publik) — dasar menampilkan tombol.
func AuthProviders() http.HandlerFunc {
	type pr struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
		URL     string `json:"url,omitempty"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		out := []pr{}
		for id, enabled := range map[string]bool{
			"google":   os.Getenv("GOOGLE_CLIENT_ID") != "" && os.Getenv("GOOGLE_CLIENT_SECRET") != "",
			"telegram": telegramOIDCEnabled(),
		} {
			p := pr{ID: id, Enabled: enabled}
			if enabled {
				p.URL = "/api/auth/" + id
			}
			out = append(out, p)
		}
		ok(w, map[string]any{
			"providers":          out,
			"google_oauth_cb":    apiURL() + "/api/auth/google/callback",
			"turnstile_site_key": turnstileSiteKey(),
		})
	}
}

type oauthStartInput struct {
	Provider       string `json:"provider"`
	AffCode        string `json:"aff_code,omitempty"`
	TurnstileToken string `json:"turnstile_token,omitempty"`
}

// OAuthStart verifies the same anti-bot challenge as password auth before
// creating the provider state/PKCE cookies. Tokens never enter a URL.
func OAuthStart(w http.ResponseWriter, r *http.Request) {
	var in oauthStartInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&in); err != nil {
		browserError(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := verifyTurnstile(r.Context(), r, in.TurnstileToken, turnstileActionAuth); err != nil {
		browserError(w, err.Error(), http.StatusForbidden)
		return
	}
	switch strings.ToLower(strings.TrimSpace(in.Provider)) {
	case "google":
		if !googleOAuthEnabled() {
			browserError(w, "google tidak aktif", http.StatusNotFound)
			return
		}
		location, err := googleStartURL(w, r, in.AffCode)
		if err != nil {
			browserError(w, "gagal membuat verifikasi login", http.StatusInternalServerError)
			return
		}
		ok(w, map[string]string{"url": location})
	case "telegram":
		if !telegramOIDCEnabled() {
			browserError(w, "telegram tidak aktif", http.StatusNotFound)
			return
		}
		location, err := telegramStartURL(w, r, in.AffCode)
		if err != nil {
			browserError(w, "gagal membuat verifikasi login", http.StatusInternalServerError)
			return
		}
		ok(w, map[string]string{"url": location})
	default:
		browserError(w, "provider tidak didukung", http.StatusBadRequest)
	}
}

// --- alur bersama -----------------------------------------------------------

func oauthState(w http.ResponseWriter, affCode string) string {
	b := make([]byte, 16)
	rand.Read(b)
	state := hex.EncodeToString(b)
	if affCode = strings.ToUpper(strings.TrimSpace(affCode)); affCode != "" {
		state += "." + base64.RawURLEncoding.EncodeToString([]byte(affCode))
	}
	http.SetCookie(w, &http.Cookie{
		Name: "oa_st", Value: state, Path: "/api/auth/",
		HttpOnly: true,
		Secure:   strings.HasPrefix(apiURL(), "https://"), // jangan Secure di http (browser menolak → state selalu gagal)
		SameSite: http.SameSiteLaxMode, MaxAge: 600,
	})
	return state
}

func oauthReferral(state string) string {
	parts := strings.SplitN(state, ".", 2)
	if len(parts) != 2 {
		return ""
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(b) > 64 {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(string(b)))
}

// clearOAuthState: buang cookie state setelah dipakai (sukses maupun gagal).
func clearOAuthState(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: "oa_st", Value: "", Path: "/api/auth/", HttpOnly: true, MaxAge: -1,
		Secure: strings.HasPrefix(apiURL(), "https://"),
	})
}

func oauthPKCE(w http.ResponseWriter, cookieName string) (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: verifier, Path: "/api/auth/", HttpOnly: true,
		Secure: strings.HasPrefix(apiURL(), "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 600,
	})
	return verifier, challenge, nil
}

func clearOAuthCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: "/api/auth/", HttpOnly: true,
		Secure: strings.HasPrefix(apiURL(), "https://"), SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

func stateOK(r *http.Request, got string) bool {
	c, err := r.Cookie("oa_st")
	return err == nil && c.Value != "" && got != "" && hmac.Equal([]byte(c.Value), []byte(got))
}

type oauthQueries interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// upsertOAuthUser binds the stable provider identity before issuing a session.
// Email is only used to migrate an older OAuth-only row or create a new user.
func upsertOAuthUser(ctx context.Context, db oauthQueries, provider, providerAccountID, email, name, ipHash string) (id, role string, created bool, err error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	providerAccountID = strings.TrimSpace(providerAccountID)
	if provider == "" || providerAccountID == "" {
		return "", "", false, fmt.Errorf("oauth provider identity kosong")
	}
	if err = db.QueryRow(ctx,
		`SELECT u.id, u.role FROM oauth_account oa JOIN "user" u ON u.id=oa.user_id
		 WHERE oa.provider=$1 AND oa.provider_account_id=$2`, provider, providerAccountID).
		Scan(&id, &role); err == nil {
		return id, role, false, nil
	} else if err != pgx.ErrNoRows {
		return "", "", false, err
	}

	email = strings.ToLower(strings.TrimSpace(email))
	err = db.QueryRow(ctx,
		`INSERT INTO "user" (email, name, role, wallet_balance, signup_ip_hash) VALUES ($1, NULLIF($2,''), 'USER', 0, NULLIF($3,''))
		 ON CONFLICT (email) DO UPDATE SET name = COALESCE("user".name, EXCLUDED.name)
		 WHERE "user".password_hash IS NULL
		 RETURNING id, role, (xmax = 0)`, email, name, ipHash).Scan(&id, &role, &created)
	if err != nil {
		return "", "", false, err
	}
	if !created && ipHash != "" {
		// Preserve the original signup IP hash for anti-abuse checks.
		_, _ = db.Exec(ctx, `UPDATE "user" SET signup_ip_hash=$2 WHERE id=$1 AND signup_ip_hash IS NULL`, id, ipHash)
	}
	if _, err = db.Exec(ctx,
		`INSERT INTO oauth_account (user_id, provider, provider_account_id)
		 VALUES ($1,$2,$3) ON CONFLICT (provider, provider_account_id) DO NOTHING`, id, provider, providerAccountID); err != nil {
		return "", "", false, err
	}
	var mappedID, mappedRole string
	if err = db.QueryRow(ctx,
		`SELECT u.id, u.role FROM oauth_account oa JOIN "user" u ON u.id=oa.user_id
		 WHERE oa.provider=$1 AND oa.provider_account_id=$2`, provider, providerAccountID).
		Scan(&mappedID, &mappedRole); err != nil {
		return "", "", false, err
	}
	if mappedID != id {
		return "", "", false, fmt.Errorf("oauth provider identity sudah terikat")
	}
	return id, mappedRole, created, nil
}

func oauthDoneWithProvider(w http.ResponseWriter, r *http.Request, db DB, provider, providerAccountID, email, name, affCode string) {
	// IP di-hash sama seperti Register: satu kolom, satu aturan anti-abuse.
	ipHash := signupIPHash(r)
	tx, err := db.Begin(r.Context())
	if err != nil {
		browserError(w, "gagal memproses login", http.StatusInternalServerError)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(r.Context())
		}
	}()
	id, role, created, err := upsertOAuthUser(r.Context(), tx, provider, providerAccountID, email, name, ipHash)
	if err != nil {
		browserError(w, "tidak bisa masuk dengan akun ini — silakan login dengan password, atau hubungi admin", http.StatusForbidden)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		browserError(w, "gagal memproses login", http.StatusInternalServerError)
		return
	}
	committed = true
	var banned bool
	if err := db.QueryRow(r.Context(), `SELECT is_banned FROM "user" WHERE id=$1`, id).Scan(&banned); err == nil && banned {
		browserError(w, "akun diblokir", http.StatusForbidden)
		return
	}
	// Akun sosial baru memakai jalur bonus/referral yang sama dengan daftar manual.
	if created {
		applySignupBenefits(r.Context(), db, id, affCode, ipHash, "oauth")
	}
	token, err := auth.IssueToken(id, role)
	if err != nil {
		browserError(w, "gagal membuat token", http.StatusInternalServerError)
		return
	}
	newSession(r, db, id, token, 24*time.Hour)
	SetSessionCookie(w, r, token)
	// Cookie HttpOnly sudah menjadi sesi; jangan menaruh JWT di URL fragment.
	http.Redirect(w, r, webURL()+"/auth/callback", http.StatusFound)
}

func oauthDone(w http.ResponseWriter, r *http.Request, db DB, email, name, affCode string) {
	oauthDoneWithProvider(w, r, db, "legacy", email, email, name, affCode)
}

func googleUserInfo(ctx context.Context, accessToken string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("userinfo status %d", res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(out)
}

// --- Google -----------------------------------------------------------------

func googleStartURL(w http.ResponseWriter, r *http.Request, affCode string) (string, error) {
	_, challenge, err := oauthPKCE(w, "ga_cv")
	if err != nil {
		return "", err
	}
	q := url.Values{
		"client_id":             {os.Getenv("GOOGLE_CLIENT_ID")},
		"response_type":         {"code"},
		"scope":                 {"openid email profile"},
		"state":                 {oauthState(w, affCode)},
		"redirect_uri":          {apiURL() + "/api/auth/google/callback"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode(), nil
}

func GoogleStart(w http.ResponseWriter, r *http.Request) {
	if turnstileEnabled() {
		browserError(w, "gunakan verifikasi keamanan sebelum login sosial", http.StatusForbidden)
		return
	}
	if !googleOAuthEnabled() {
		browserError(w, "google tidak aktif", http.StatusNotFound)
		return
	}
	location, err := googleStartURL(w, r, r.URL.Query().Get("aff"))
	if err != nil {
		browserError(w, "gagal membuat verifikasi login", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, location, http.StatusFound)
}

func GoogleCallback(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clearOAuthState(w)
		cv, cvErr := r.Cookie("ga_cv")
		clearOAuthCookie(w, "ga_cv")
		state := r.URL.Query().Get("state")
		if !googleOAuthEnabled() || !stateOK(r, state) || cvErr != nil || cv.Value == "" || r.URL.Query().Get("code") == "" {
			browserError(w, "state oauth salah", http.StatusBadRequest)
			return
		}
		q := url.Values{
			"client_id":     {os.Getenv("GOOGLE_CLIENT_ID")},
			"client_secret": {os.Getenv("GOOGLE_CLIENT_SECRET")},
			"code":          {r.URL.Query().Get("code")},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {apiURL() + "/api/auth/google/callback"},
			"code_verifier": {cv.Value},
		}
		req, _ := http.NewRequestWithContext(r.Context(), "POST", "https://oauth2.googleapis.com/token", strings.NewReader(q.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
		if err != nil {
			browserError(w, "google token gagal", http.StatusBadGateway)
			return
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			browserError(w, "google token gagal", http.StatusBadGateway)
			return
		}
		var tok struct {
			AccessToken string `json:"access_token"`
		}
		json.NewDecoder(res.Body).Decode(&tok)
		if tok.AccessToken == "" {
			browserError(w, "google token kosong", http.StatusBadGateway)
			return
		}
		var me struct {
			Sub      string `json:"sub"`
			Email    string `json:"email"`
			Verified bool   `json:"email_verified"`
			Name     string `json:"name"`
		}
		if err := googleUserInfo(r.Context(), tok.AccessToken, &me); err != nil || me.Sub == "" || me.Email == "" || !me.Verified {
			browserError(w, "email google belum terverifikasi", http.StatusBadRequest)
			return
		}
		oauthDoneWithProvider(w, r, db, "google", me.Sub, me.Email, me.Name, oauthReferral(state))
	}
}

// --- Telegram Login (OIDC) ---
// Alur authorization-code ada di telegram_oidc.go; widget HMAC lama sudah
// dihapus karena tombolnya tidak bisa mengikuti tema web dan memblokir login
// dengan nomor telepon manual.
