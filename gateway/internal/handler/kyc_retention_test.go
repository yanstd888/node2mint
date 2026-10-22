package handler

import (
	"strings"
	"testing"
)

func TestKYCVerificationRetentionSQL(t *testing.T) {
	sql := strings.ToLower(pruneKYCVerificationSQL)
	for _, want := range []string{
		"delete from kyc_verification",
		"status in ('approved','rejected')",
		"reviewed_at < now() - interval '30 days'",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("KYC retention SQL missing %q: %s", want, pruneKYCVerificationSQL)
		}
	}
}
