package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueTokenGetsUniqueSessionIdentity(t *testing.T) {
	first, err := IssueToken("user-a", "USER")
	if err != nil {
		t.Fatal(err)
	}
	second, err := IssueToken("user-a", "USER")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("tokens issued in the same second must still be unique")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(first, claims, func(*jwt.Token) (any, error) {
		return []byte(Secret), nil
	})
	if err != nil || !token.Valid || claims.ID == "" {
		t.Fatalf("token JTI missing or invalid: valid=%t id=%q err=%v", token.Valid, claims.ID, err)
	}
}
