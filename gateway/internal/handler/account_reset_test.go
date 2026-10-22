package handler

import (
	"os"
	"strings"
	"testing"
)

func TestPasswordResetTokenStoresOnlyDigest(t *testing.T) {
	raw, digest, err := newPasswordResetToken()
	if err != nil {
		t.Fatalf("newPasswordResetToken() error = %v", err)
	}
	if raw == "" || len(raw) < 40 {
		t.Fatalf("raw token too short: %q", raw)
	}
	if digest != passwordResetTokenHash(raw) {
		t.Fatal("stored digest must be derived from the raw token")
	}
	if len(digest) != 64 || strings.Contains(digest, raw) {
		t.Fatalf("digest must be SHA-256 hex, got %q", digest)
	}
}

func TestPasswordResetEmailPayloadUsesPublicResetURL(t *testing.T) {
	t.Setenv("PUBLIC_WEB_URL", "https://www.node2mint.com")
	payload, err := passwordResetEmailPayload("owner@example.test", "Owner", "en", "token-value")
	if err != nil {
		t.Fatalf("passwordResetEmailPayload() error = %v", err)
	}
	if payload.To != "owner@example.test" || payload.Locale != "en" || payload.RecipientName != "Owner" {
		t.Fatalf("payload recipient = %#v", payload)
	}
	for key, want := range map[string]string{
		"customer_name": "Owner",
		"reset_url":     "https://www.node2mint.com/reset-password?token=token-value",
		"support_url":   "https://www.node2mint.com/contact",
	} {
		if got, _ := payload.Params[key].(string); got != want {
			t.Fatalf("params[%q] = %q, want %q", key, got, want)
		}
	}
	if got, _ := payload.Params["expires_minutes"].(int); got != int(passwordResetTTL.Minutes()) {
		t.Fatalf("expires_minutes = %d", got)
	}
}

func TestPasswordResetStorageAndConsumptionAreAtomic(t *testing.T) {
	schema, err := os.ReadFile("../../sql/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(schema), "uq_verification_token_reset_identifier") {
		t.Fatal("schema must allow only one active reset token per recipient")
	}
	source, err := os.ReadFile("account.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"ON CONFLICT (identifier) WHERE purpose='RESET_PASSWORD'",
		"passwordResetTokenHash(in.Token)",
		"DELETE FROM verification_token\n			WHERE token=$1 AND purpose='RESET_PASSWORD' AND expires > now()\n			RETURNING identifier",
		"context.Background()",
		"}(\"password-reset:\"+digest, payload)",
	} {
		if !strings.Contains(string(source), want) {
			t.Fatalf("account.go missing %q", want)
		}
	}
}
