// Auth: JWT Bearer token. Satu file — issue + verify.
// Ponytail: tidak ada refresh token di v1; token exp 24h cukup untuk dev.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// Secret wajib dari env; validasi startup dilakukan oleh main().
	Secret string = os.Getenv("JWT_SECRET")
)

// Validate dipanggil main() sebelum server listen. Panjang minimum tetap
// ditegakkan di server supaya paket ini tetap bisa diimpor oleh test tanpa
// proses yang gagal saat impor.
func Validate() error {
	if len(Secret) < 32 {
		return fmt.Errorf("JWT_SECRET minimal 32 karakter (sekarang %d)", len(Secret))
	}
	return nil
}

type Claims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// IssueToken buat user login.
func IssueToken(userID, role string) (string, error) {
	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}
	c := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        hex.EncodeToString(jtiBytes),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString([]byte(Secret))
}

// VerifyToken parse + validasi.
func VerifyToken(token string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		return []byte(Secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
