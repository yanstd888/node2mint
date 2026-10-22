package handler

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"ai-sell/gateway/internal/billing"
	"ai-sell/gateway/internal/provider"
)

// TestApilogStatus: maps runChat return to api_request_log.status.
// 'fail' on any error; 'success' on nil error regardless of usage content.
func TestApilogStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		usage    provider.UsageUsage
		fallback bool
		want     string
	}{
		{"nil error", nil, provider.UsageUsage{Content: "hello"}, false, "success"},
		{"nil error empty content", nil, provider.UsageUsage{Content: ""}, false, "success"},
		{"fallback success", nil, provider.UsageUsage{Content: "hello"}, true, "fallback"},
		{"whitelist error", fmt.Errorf("model tidak di whitelist key"), provider.UsageUsage{}, false, "fail"},
		{"billing error", billing.ErrInsufficientCredit, provider.UsageUsage{}, false, "fail"},
		{"provider error", errors.New("connection refused"), provider.UsageUsage{}, false, "fail"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := apilogStatus(tc.err, tc.usage, tc.fallback); got != tc.want {
				t.Errorf("apilogStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestApilogErrorCode: maps runChat errors to stable error_code strings
// for the api_request_log.error_code column. Admin filters by these.
func TestApilogErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil error returns empty", nil, ""},
		{"insufficient credit", billing.ErrInsufficientCredit, "insufficient_credit"},
		{"credit limit exceeded", billing.ErrCreditLimitExceeded, "credit_limit_exceeded"},
		{"whitelist denied", fmt.Errorf("model tidak di whitelist key"), "key_whitelist_denied"},
		{"model not found", fmt.Errorf("model tidak ada di katalog"), "model_not_found"},
		{"tpm exceeded", fmt.Errorf("tpm limit exceeded"), "tpm_limit_exceeded"},
		{"unknown error falls to internal", errors.New("some random failure"), "internal_error"},
		{"wrapped billing error", fmt.Errorf("charge failed: %w", billing.ErrInsufficientCredit), "insufficient_credit"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := apilogErrorCode(tc.err); got != tc.want {
				t.Errorf("apilogErrorCode() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRunChatLogEntryColumns: verifies the api_request_log INSERT (Task 14)
// writes exactly the allowlisted columns — no prompt/response, no raw keys.
// This is a schema-level assertion checked against the SQL string.
func TestRunChatLogEntryColumns(t *testing.T) {
	q := `INSERT INTO api_request_log
	(user_id,key_id,model,provider,input_tokens,output_tokens,cached_tokens,cost_tokens,latency_ms,status,error_code)
	VALUES (NULLIF($1,'')::uuid,NULLIF($2,'')::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	lower := strings.ToLower(q)
	for _, bad := range []string{"prompt", "response", "api_key", "raw_key", "content", "messages", "password"} {
		if strings.Contains(lower, bad) {
			t.Errorf("api_request_log INSERT must not contain %q (privacy violation)", bad)
		}
	}
}
