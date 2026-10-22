package secret

// Jalur 2FA login: setelah user mengaktifkan 2FA, satu-satunya cara masuk adalah
// mengirim kode TOTP yang benar. Kalau verifikasi ini salah, akun terkunci
// permanen — jadi diuji terhadap vektor resmi RFC 6238.
import (
	"testing"
	"time"
)

// Vektor RFC 6238 Appendix B (HMAC-SHA1, secret ASCII "12345678901234567890",
// 8 digit). Implementasi di sini memakai 6 digit, jadi yang dibandingkan adalah
// 6 digit terakhir dari nilai vektor.
func TestTOTPMatchesRFC6238Vectors(t *testing.T) {
	// base32 dari ASCII "12345678901234567890"
	const sec = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	cases := []struct {
		unix int64
		want string // 6 digit terakhir dari vektor 8 digit RFC
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, c := range cases {
		got, err := totpAt(sec, uint64(c.unix/30))
		if err != nil {
			t.Fatalf("totpAt(%d): %v", c.unix, err)
		}
		if got != c.want {
			t.Errorf("T=%d: got %s, want %s", c.unix, got, c.want)
		}
	}
}

func TestVerifyTOTPAcceptsCurrentCode(t *testing.T) {
	sec, err := NewTOTPSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	code, err := totpAt(sec, uint64(time.Now().Unix()/30))
	if err != nil {
		t.Fatalf("totpAt: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("want 6 digits, got %q", code)
	}
	if !VerifyTOTP(sec, code) {
		t.Fatalf("current code %q rejected — 2FA login impossible", code)
	}
	// Tempelan dari clipboard sering membawa spasi.
	if !VerifyTOTP(sec, " "+code+" ") {
		t.Error("code with surrounding spaces rejected")
	}
}

func TestVerifyTOTPRejectsWrongAndMalformed(t *testing.T) {
	sec, _ := NewTOTPSecret()
	if VerifyTOTP(sec, "000000") {
		t.Error("all-zero code accepted")
	}
	for _, bad := range []string{"", "12345", "1234567", "abcdef", "  "} {
		if VerifyTOTP(sec, bad) {
			t.Errorf("malformed code %q accepted", bad)
		}
	}
}
