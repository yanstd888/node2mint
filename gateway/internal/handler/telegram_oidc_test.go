package handler

// Cek regresi jalur verifikasi id_token Telegram OIDC tanpa approval manual:
// server JWKS palsu lokal, lalu tanda tangani token dengan tiap algoritma yang
// diiklankan Telegram (RS256/ES256/EdDSA) dan pastikan diterima; ES256K harus
// ditolak karena kurvanya di luar stdlib.
import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func TestTelegramPublicKeyAllAdvertisedAlgorithms(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	edPub, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_ = edPub

	rsaN := rsaKey.PublicKey.N.Bytes()
	keys := []telegramJWK{
		{KID: "r1", KTY: "RSA", N: b64(rsaN), E: b64(big.NewInt(int64(rsaKey.PublicKey.E)).Bytes())},
		{KID: "e1", KTY: "EC", Crv: "P-256", X: b64(ecKey.PublicKey.X.Bytes()), Y: b64(ecKey.PublicKey.Y.Bytes())},
		{KID: "o1", KTY: "OKP", Crv: "Ed25519", X: b64(edPriv.Public().(ed25519.PublicKey))},
		{KID: "k1", KTY: "EC", Crv: "secp256k1", X: b64([]byte{1, 2, 3}), Y: b64([]byte{4, 5, 6})},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(telegramJWKS{Keys: keys})
	}))
	defer srv.Close()

	saved := telegramIssuer
	telegramIssuer = srv.URL
	defer func() { telegramIssuer = saved }()

	os.Setenv("TELEGRAM_CLIENT_ID", "123456")
	defer os.Unsetenv("TELEGRAM_CLIENT_ID")

	claims := &telegramClaims{ID: "987654321", Name: "John Doe", Username: "johndoe"}
	claims.Issuer = telegramIssuer
	claims.Audience = jwt.ClaimStrings{"123456"}
	cases := []struct {
		kid string
		m   jwt.SigningMethod
		key any
	}{
		{"r1", jwt.SigningMethodRS256, rsaKey},
		{"e1", jwt.SigningMethodES256, ecKey},
		{"o1", jwt.SigningMethodEdDSA, edPriv},
	}
	for _, c := range cases {
		tok := jwt.NewWithClaims(c.m, claims)
		tok.Header["kid"] = c.kid
		signed, err := tok.SignedString(c.key)
		if err != nil {
			t.Fatalf("%s sign: %v", c.m.Alg(), err)
		}
		got := &telegramClaims{}
		parsed, err := jwt.ParseWithClaims(signed, got, func(tk *jwt.Token) (any, error) {
			return telegramKey(httptest.NewRequest(http.MethodGet, "/", nil), tk)
		}, jwt.WithValidMethods([]string{"RS256", "ES256", "EdDSA"}), jwt.WithIssuer(telegramIssuer), jwt.WithAudience("123456"))
		if err != nil || !parsed.Valid || got.telegramUserID() != "987654321" {
			t.Fatalf("%s harus valid, err=%v id=%s", c.m.Alg(), err, got.ID)
		}
	}

	// ES256K: JWKS ada, tapi kurvanya harus ditolak.
	if _, err := telegramPublicKey(keys[3]); err == nil {
		t.Fatal("kurva secp256k1 harus ditolak")
	}
	// Klaim `id` Telegram berupa string dan bisa > int64 (sub 1.57e19) —
	// token nyata harus ter-decode, bukan gagal unmarshal.
	big := jwt.NewWithClaims(jwt.SigningMethodRS256, &telegramClaims{ID: "15723026036826245096"})
	big.Header["kid"] = "r1"
	signedBig, err := big.SignedString(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	gotBig := &telegramClaims{}
	if _, err := jwt.ParseWithClaims(signedBig, gotBig, func(tk *jwt.Token) (any, error) {
		return telegramKey(httptest.NewRequest(http.MethodGet, "/", nil), tk)
	}, jwt.WithValidMethods([]string{"RS256"})); err != nil {
		t.Fatalf("id string besar harus ter-decode: %v", err)
	}
	if gotBig.telegramUserID() != "15723026036826245096" {
		t.Fatalf("uid salah: %s", gotBig.telegramUserID())
	}
	// id non-numerik harus ditolak.
	bad := jwt.NewWithClaims(jwt.SigningMethodRS256, &telegramClaims{ID: "abc"})
	bad.Header["kid"] = "r1"
	signedBad, err := bad.SignedString(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jwt.ParseWithClaims(signedBad, &telegramClaims{}, func(tk *jwt.Token) (any, error) {
		return telegramKey(httptest.NewRequest(http.MethodGet, "/", nil), tk)
	}, jwt.WithValidMethods([]string{"RS256"})); err == nil {
		t.Fatal("id non-numerik harus ditolak")
	}
	// Algoritma simetris tidak boleh diterima.
	none := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	none.Header = map[string]any{"alg": "HS256", "kid": "r1"}
	if _, err := telegramKey(httptest.NewRequest(http.MethodGet, "/", nil), none); err == nil {
		t.Fatal("HS256 harus ditolak")
	}
	// kid tidak dikenal harus gagal, bukan diam-diam pakai kunci lain.
	missing := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	missing.Header["kid"] = "tidak-ada"
	if _, err := telegramKey(httptest.NewRequest(http.MethodGet, "/", nil), missing); err == nil {
		t.Fatal("kid tidak dikenal harus ditolak")
	}
}
