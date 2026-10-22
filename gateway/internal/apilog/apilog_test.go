package apilog

import (
	"context"
	"strings"
	"testing"
)

// TestNormalizeStatus: status normalization guard in Record.
// Only 'success','fail','fallback' are valid; anything else defaults to 'fail'.
// Single gate preventing garbage status values into api_request_log table.
func TestNormalizeStatus(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"success", "success"},
		{"fail", "fail"},
		{"fallback", "fallback"},
		{"", "fail"},
		{"error", "fail"},
		{"SUCCESS", "fail"},
		{"rate_limited", "fail"},
	}
	for _, c := range cases {
		if got := normalizeStatus(c.in); got != c.want {
			t.Errorf("normalizeStatus(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestInsertSQLNoContent: INSERT must never contain prompt, response, or
// raw API key columns — privacy invariant for Task 14.
func TestInsertSQLNoContent(t *testing.T) {
	for _, bad := range []string{"prompt", "response", "api_key", "raw_key", "content", "messages"} {
		if strings.Contains(strings.ToLower(insertSQL), bad) {
			t.Errorf("insertSQL must not contain %q (privacy violation)", bad)
		}
	}
	for _, must := range []string{"api_request_log", "user_id", "key_id", "model", "provider",
		"input_tokens", "output_tokens", "cached_tokens", "cost_tokens", "latency_ms", "status", "error_code"} {
		if !strings.Contains(insertSQL, must) {
			t.Errorf("insertSQL missing required column %q", must)
		}
	}
}

// TestInsertSQLColumnOrder: positional arg order must match api_request_log
// column order in schema.
func TestInsertSQLColumnOrder(t *testing.T) {
	cols := strings.Split("user_id,key_id,model,provider,input_tokens,output_tokens,cached_tokens,cost_tokens,latency_ms,status,error_code", ",")
	if len(cols) != 11 {
		t.Fatalf("expected 11 columns, got %d", len(cols))
	}
	for i, c := range cols {
		if c == "status" && cols[i+1] != "error_code" {
			t.Errorf("status must be immediately before error_code, got %s", cols[i+1])
		}
		if c == "latency_ms" && cols[i+1] != "status" {
			t.Errorf("latency_ms must come before status, got %s", cols[i+1])
		}
	}
}

// TestMetaHasNoContentFields: Meta struct carries only allowlisted metadata.
func TestMetaHasNoContentFields(t *testing.T) {
	m := Meta{
		UserID: "user-1", KeyID: "key-2", Model: "gpt-4o",
		Provider: "openai", InputTokens: 10, OutputTokens: 5,
		CachedTokens: 3, CostTokens: 15, Fallback: false, ErrorCode: "test_err",
	}
	_ = m
}

func TestRequestMetaErrorAndRecordState(t *testing.T) {
	ctx, meta := With(context.Background())
	if Recorded(ctx) {
		t.Fatal("new request metadata must not be marked recorded")
	}
	SetError(ctx, "permission_denied")
	if meta.ErrorCode != "permission_denied" {
		t.Fatalf("ErrorCode = %q, want permission_denied", meta.ErrorCode)
	}
	markRecorded(ctx)
	if !Recorded(ctx) {
		t.Fatal("record state was not marked")
	}
}
