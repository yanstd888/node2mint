package secret

import (
	"encoding/base32"
	"os"
	"strings"
	"testing"
)

// Jalankan dengan SECRET_KEY di-set; tanpa itu, uji enkripsi dilewati karena
// tidak ada kunci untuk dipakai (bukan kegagalan kode).
func TestEncryptDecryptRoundTrip(t *testing.T) {
	if len(os.Getenv("SECRET_KEY")) < 32 {
		t.Skip("SECRET_KEY belum diset")
	}
	for _, plain := range []string{"sk-proj-abcdef1234567890", "a", strings.Repeat("x", 400)} {
		ct, err := Encrypt(plain)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plain, err)
		}
		if !strings.HasPrefix(ct, "v1.") {
			t.Fatalf("ciphertext tidak berformat v1.: %q", ct)
		}
		if strings.Contains(ct, plain) && plain != "a" {
			t.Fatal("ciphertext memuat plaintext")
		}
		got, err := Decrypt(ct)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if got != plain {
			t.Fatalf("round-trip = %q, mau %q", got, plain)
		}
	}
}

// Nilai lama yang belum terenkripsi harus tetap lewat — kalau tidak, provider
// yang dikonfigurasi sebelum enkripsi dipasang langsung mati.
func TestDecryptLegacyPlaintext(t *testing.T) {
	got, err := Decrypt("sk-legacy-plaintext")
	if err != nil || got != "sk-legacy-plaintext" {
		t.Fatalf("legacy = %q, %v", got, err)
	}
}

func TestMask(t *testing.T) {
	if got := Mask("sk-abcdef1234"); got != "••••1234" {
		t.Fatalf("Mask = %q", got)
	}
	if got := Mask("abc"); got != "••••" {
		t.Fatalf("Mask pendek = %q", got)
	}
	if got := Mask(""); got != "" {
		t.Fatalf("Mask kosong = %q", got)
	}
}

// Vektor uji RFC 6238 Appendix B, versi 6 digit (kode mod 10^6).
func TestTOTPRFC6238Vectors(t *testing.T) {
	const ascii = "12345678901234567890"
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(ascii))
	for _, tc := range []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	} {
		got, err := totpAt(secret, uint64(tc.unix/30))
		if err != nil {
			t.Fatalf("totpAt(%d): %v", tc.unix, err)
		}
		if got != tc.want {
			t.Fatalf("T=%d: dapat %s, mau %s", tc.unix, got, tc.want)
		}
	}
	// Kode salah bentuk / salah digit tidak boleh lolos.
	if VerifyTOTP("bogus!!", "12345") {
		t.Fatal("kode 5 digit lolos")
	}
}
