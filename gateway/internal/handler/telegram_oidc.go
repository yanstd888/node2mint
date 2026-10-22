package handler

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// var, bukan const: test menukarnya ke server JWKS lokal.
var telegramIssuer = "https://oauth.telegram.org"

func telegramOIDCEnabled() bool {
	return os.Getenv("TELEGRAM_CLIENT_ID") != "" && os.Getenv("TELEGRAM_CLIENT_SECRET") != ""
}

func telegramCallbackURL() string { return apiURL() + "/api/auth/telegram/callback" }

func telegramPKCE(w http.ResponseWriter) (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	http.SetCookie(w, &http.Cookie{Name: "tg_cv", Value: verifier, Path: "/api/auth/", HttpOnly: true, Secure: strings.HasPrefix(apiURL(), "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 600})
	return verifier, challenge, nil
}

func clearTelegramPKCE(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "tg_cv", Value: "", Path: "/api/auth/", HttpOnly: true, Secure: strings.HasPrefix(apiURL(), "https://"), SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

func telegramStartURL(w http.ResponseWriter, r *http.Request, affCode string) (string, error) {
	_, challenge, err := telegramPKCE(w)
	if err != nil {
		return "", err
	}
	q := url.Values{
		"client_id":             {os.Getenv("TELEGRAM_CLIENT_ID")},
		"redirect_uri":          {telegramCallbackURL()},
		"response_type":         {"code"},
		"scope":                 {"openid profile"},
		"state":                 {oauthState(w, affCode)},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return telegramIssuer + "/auth?" + q.Encode(), nil
}

func TelegramStart(w http.ResponseWriter, r *http.Request) {
	if turnstileEnabled() {
		browserError(w, "gunakan verifikasi keamanan sebelum login sosial", http.StatusForbidden)
		return
	}
	if !telegramOIDCEnabled() {
		browserError(w, "telegram tidak aktif", http.StatusNotFound)
		return
	}
	location, err := telegramStartURL(w, r, r.URL.Query().Get("aff"))
	if err != nil {
		browserError(w, "gagal membuat verifikasi login", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, location, http.StatusFound)
}

type telegramJWK struct {
	KID string `json:"kid"`
	KTY string `json:"kty"`
	Crv string `json:"crv"`
	N   string `json:"n"`
	E   string `json:"e"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

type telegramJWKS struct {
	Keys []telegramJWK `json:"keys"`
}

// telegramID: Telegram mengirim klaim id/sub sebagai string JSON dan nilainya
// bisa melewati batas int64 (sub sering > 9.2e18), jadi simpan sebagai string
// setelah dipastikan numerik.
type telegramID string

func (t *telegramID) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" {
		s = ""
	}
	if s != "" {
		if _, err := strconv.ParseUint(s, 10, 64); err != nil {
			return fmt.Errorf("id telegram bukan angka: %s", s)
		}
	}
	*t = telegramID(s)
	return nil
}

type telegramClaims struct {
	ID       telegramID `json:"id"`
	Name     string     `json:"name"`
	Username string     `json:"preferred_username"`
	jwt.RegisteredClaims
}

// telegramUserID: klaim `id` numerik, fallback ke `sub` (id Telegram sama).
func (c *telegramClaims) telegramUserID() string {
	if c.ID != "" {
		return string(c.ID)
	}
	if _, err := strconv.ParseUint(c.Subject, 10, 64); err == nil {
		return c.Subject
	}
	return ""
}

// telegramAlgs: algoritma asimetris yang diiklankan JWKS Telegram dan bisa
// diverifikasi stdlib. ES256K sengaja dilewati (butuh kurva secp256k1, bukan
// stdlib) — Telegram tidak menandatangani id_token dengan itu.
var telegramAlgs = map[string]bool{"RS256": true, "ES256": true, "EdDSA": true}

// telegramPublicKey mengubah satu entri JWKS jadi public key stdlib. Murni,
// tanpa jaringan, supaya bisa diuji tanpa approval Telegram manual.
func telegramPublicKey(k telegramJWK) (any, error) {
	raw := base64.RawURLEncoding.DecodeString
	switch k.KTY {
	case "RSA":
		n, err := raw(k.N)
		if err != nil {
			return nil, err
		}
		e, err := raw(k.E)
		if err != nil {
			return nil, err
		}
		if len(e) == 0 || len(n) == 0 {
			return nil, fmt.Errorf("modulus jwks kosong")
		}
		exp := 0
		for _, v := range e {
			exp = exp<<8 | int(v)
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exp}, nil
	case "EC":
		if k.Crv != "P-256" {
			return nil, fmt.Errorf("kurva jwks tidak didukung: %s", k.Crv)
		}
		x, err := raw(k.X)
		if err != nil {
			return nil, err
		}
		y, err := raw(k.Y)
		if err != nil {
			return nil, err
		}
		if len(x) == 0 || len(y) == 0 {
			return nil, fmt.Errorf("titik jwks kosong")
		}
		return &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, nil
	case "OKP":
		if k.Crv != "Ed25519" {
			return nil, fmt.Errorf("kurva jwks tidak didukung: %s", k.Crv)
		}
		x, err := raw(k.X)
		if err != nil {
			return nil, err
		}
		if len(x) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("panjang kunci ed25519 salah: %d", len(x))
		}
		return ed25519.PublicKey(x), nil
	}
	return nil, fmt.Errorf("kty jwks tidak didukung: %s", k.KTY)
}

func telegramKey(ctxReq *http.Request, token *jwt.Token) (any, error) {
	if !telegramAlgs[token.Method.Alg()] {
		return nil, fmt.Errorf("algoritma token telegram tidak didukung: %s", token.Method.Alg())
	}
	kid, _ := token.Header["kid"].(string)
	if kid == "" {
		return nil, fmt.Errorf("kid token telegram kosong")
	}
	req, err := http.NewRequestWithContext(ctxReq.Context(), http.MethodGet, telegramIssuer+"/.well-known/jwks.json", nil)
	if err != nil {
		return nil, err
	}
	// ponytail: JWKS di-fetch tiap callback; cache kalau login jadi sering.
	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks telegram status %d", res.StatusCode)
	}
	var keys telegramJWKS
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&keys); err != nil {
		return nil, err
	}
	for _, key := range keys.Keys {
		if key.KID == kid {
			return telegramPublicKey(key)
		}
	}
	return nil, fmt.Errorf("kunci telegram tidak ditemukan: %s", kid)
}

func TelegramCallback(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		cv, err := r.Cookie("tg_cv")
		clearOAuthState(w)
		clearTelegramPKCE(w)
		if !telegramOIDCEnabled() || !stateOK(r, state) || err != nil || cv.Value == "" || r.URL.Query().Get("code") == "" {
			browserError(w, "state telegram salah", http.StatusBadRequest)
			return
		}
		form := url.Values{"grant_type": {"authorization_code"}, "code": {r.URL.Query().Get("code")}, "redirect_uri": {telegramCallbackURL()}, "client_id": {os.Getenv("TELEGRAM_CLIENT_ID")}, "code_verifier": {cv.Value}}
		req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, telegramIssuer+"/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(os.Getenv("TELEGRAM_CLIENT_ID"), os.Getenv("TELEGRAM_CLIENT_SECRET"))
		res, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
		if err != nil || res.StatusCode != http.StatusOK {
			body := ""
			if res != nil {
				b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
				body = string(b)
				res.Body.Close()
			}
			log.Printf("[WARN] telegram oidc token exchange gagal: err=%v body=%s", err, body)
			browserError(w, "token telegram gagal", http.StatusBadGateway)
			return
		}
		var out struct {
			IDToken string `json:"id_token"`
		}
		err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out)
		res.Body.Close()
		if err != nil || out.IDToken == "" {
			log.Printf("[WARN] telegram oidc id_token kosong: err=%v", err)
			browserError(w, "id token telegram kosong", http.StatusBadGateway)
			return
		}
		claims := &telegramClaims{}
		parsed, err := jwt.ParseWithClaims(out.IDToken, claims, func(t *jwt.Token) (any, error) { return telegramKey(r, t) }, jwt.WithValidMethods([]string{"RS256", "ES256", "EdDSA"}), jwt.WithIssuer(telegramIssuer), jwt.WithAudience(os.Getenv("TELEGRAM_CLIENT_ID")))
		uid := claims.telegramUserID()
		if err != nil || !parsed.Valid || uid == "" {
			// Tanpa diagnosa ini kegagalan verifikasi cuma jadi pesan generik,
			// dan tiap percobaan butuh approval Telegram manual lagi.
			alg, kid := "", ""
			if parsed != nil {
				alg, _ = parsed.Header["alg"].(string)
				kid, _ = parsed.Header["kid"].(string)
			}
			log.Printf("[WARN] telegram oidc id token ditolak: err=%v alg=%s kid=%s iss=%s aud=%v sub=%s id=%s", err, alg, kid, claims.Issuer, claims.Audience, claims.Subject, claims.ID)
			browserError(w, "id token telegram tidak valid", http.StatusUnauthorized)
			return
		}
		name := claims.Name
		if name == "" {
			name = claims.Username
		}
		oauthDoneWithProvider(w, r, db, "telegram", string(uid), fmt.Sprintf("tg-%s@telegram.local", uid), name, oauthReferral(state))
	}
}
