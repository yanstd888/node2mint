// Rahasia yang disimpan di DB tidak boleh apa adanya: kunci API provider
// (provider.api_key) dan secret TOTP (user_auth_method.secret) lewat AES-256-GCM
// dengan kunci dari env SECRET_KEY. Format: v1.<iv>.<ciphertext+tag> (base64url).
//
// TOTP ikut di sini karena satu-satunya alasan secret TOTP butuh paket ini
// adalah enkripsinya — memisahkannya hanya menambah impor.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// ErrNoKey muncul saat SECRET_KEY belum diset. Pemanggil yang membaca nilai
// lama (plaintext) tetap boleh lanjut; yang menulis nilai baru harus gagal.
var ErrNoKey = errors.New("SECRET_KEY belum diset (minimal 32 byte)")

func key() ([]byte, error) {
	raw := os.Getenv("SECRET_KEY")
	if len(raw) < 32 {
		return nil, ErrNoKey
	}
	sum := sha256.Sum256([]byte(raw))
	return sum[:], nil
}

// Validate: fail-fast untuk produksi — SECRET_KEY kosong/pendek harus membuat
// server menolak hidup, bukan diam-diam menolak menyimpan kunci baru.
func Validate() error {
	if _, err := key(); err != nil {
		return err
	}
	return nil
}

// Encrypt: plaintext -> "v1.<iv>.<ct>".
func Encrypt(plain string) (string, error) {
	k, err := key()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, iv, []byte(plain), nil)
	return "v1." + base64.RawURLEncoding.EncodeToString(iv) + "." +
		base64.RawURLEncoding.EncodeToString(ct), nil
}

// Decrypt mengembalikan plaintext. Nilai tanpa awalan "v1." diperlakukan
// sebagai nilai lama yang belum terenkripsi — dibiarkan lewat supaya baris yang
// sudah ada sebelum enkripsi dipasang tidak langsung mati.
func Decrypt(stored string) (string, error) {
	if stored == "" {
		return "", errors.New("nilai kosong")
	}
	if !strings.HasPrefix(stored, "v1.") {
		return stored, nil
	}
	parts := strings.SplitN(stored, ".", 3)
	if len(parts) != 3 {
		return "", errors.New("format rahasia tidak dikenali")
	}
	k, err := key()
	if err != nil {
		return "", err
	}
	iv, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errors.New("iv tidak valid")
	}
	ct, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", errors.New("ciphertext tidak valid")
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	pt, err := gcm.Open(nil, iv, ct, nil)
	if err != nil {
		return "", errors.New("gagal dekripsi (kunci salah?)")
	}
	return string(pt), nil
}

// Mask dipakai untuk menampilkan kunci di API — nilai penuh tidak pernah keluar.
func Mask(plain string) string {
	if plain == "" {
		return ""
	}
	if len(plain) <= 4 {
		return "••••"
	}
	return "••••" + plain[len(plain)-4:]
}

// ------------------------------------------------------------------ TOTP
// RFC 6238/4226: 6 digit, 30 detik, HMAC-SHA1. Tanpa dependensi baru.

const (
	totpPeriod = 30
	totpDigits = 6
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret: 20 byte acak (160 bit, panjang yang disarankan RFC 4226).
func NewTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return b32.EncodeToString(buf), nil
}

func decodeSecret(s string) ([]byte, error) {
	clean := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	clean = strings.TrimRight(clean, "=")
	if clean == "" {
		return nil, errors.New("secret kosong")
	}
	return b32.DecodeString(clean)
}

// totpAt: HOTP pada langkah waktu ke-n.
func totpAt(secret string, counter uint64) (string, error) {
	kb, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, kb)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[off]&0x7f) << 24) | (uint32(sum[off+1]) << 16) |
		(uint32(sum[off+2]) << 8) | uint32(sum[off+3])
	return fmt.Sprintf("%06d", code%1000000), nil
}

// VerifyTOTP: toleransi ±1 langkah karena jam perangkat sering meleset.
// Tidak ada penolakan pemakaian ulang: aplikasi authenticator menampilkan kode
// yang sama selama 30 detik, jadi kode kedua yang sah akan ikut ditolak.
func VerifyTOTP(secret, code string) bool {
	clean := strings.TrimSpace(code)
	if len(clean) != totpDigits {
		return false
	}
	step := uint64(time.Now().Unix() / totpPeriod)
	for d := int64(-1); d <= 1; d++ {
		want, err := totpAt(secret, uint64(int64(step)+d))
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(clean)) == 1 {
			return true
		}
	}
	return false
}

// OTPAuthURL: untuk QR authenticator (Google Authenticator, Authy, 1Password).
func OTPAuthURL(secret, account, issuer string) string {
	label := account
	if issuer != "" {
		label = issuer + ":" + account
	}
	return fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		strings.ReplaceAll(label, " ", "%20"), secret, issuer, totpDigits, totpPeriod)
}
