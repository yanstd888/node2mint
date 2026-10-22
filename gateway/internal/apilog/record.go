package apilog

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Record persists allowlisted metadata only; callers intentionally ignore errors.
func Record(ctx context.Context, db *pgxpool.Pool, m Meta, latency int, status string) error {
	s := normalizeStatus(status)
	_, err := db.Exec(ctx, insertSQL,
		m.UserID, m.KeyID, m.Model, m.Provider, m.InputTokens, m.OutputTokens,
		m.CachedTokens, m.CostTokens, latency, s, m.ErrorCode)
	if err == nil {
		markRecorded(ctx)
	}
	return err
}

// normalizeStatus maps any status string to one of the three schema-allowed
// values ('success','fail','fallback'). Anything unrecognized defaults to
// 'fail' so Task 14 admin queries always see a valid CHECK value.
func normalizeStatus(s string) string {
	if s != "success" && s != "fail" && s != "fallback" {
		return "fail"
	}
	return s
}

// insertSQL is the allowlisted INSERT for api_request_log. It must NOT contain
// prompt, response, or raw API key columns — verified by TestInsertSQLNoContent.
const insertSQL = `INSERT INTO api_request_log
	(user_id,key_id,model,provider,input_tokens,output_tokens,cached_tokens,cost_tokens,latency_ms,status,error_code)
	VALUES (NULLIF($1,'')::uuid,NULLIF($2,'')::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11)`

// ErrCode constants for api_request_log.error_code column. These are the
// stable, machine-readable reasons admin can filter by in /admin/requests.
const (
	ErrCodeWhitelistDenied     = "key_whitelist_denied"
	ErrCodeModelNotFound       = "model_not_found"
	ErrCodeTPMExceeded         = "tpm_limit_exceeded"
	ErrCodeInsufficientCredit  = "insufficient_credit"
	ErrCodeCreditLimitExceeded = "credit_limit_exceeded"
	ErrCodeUpstreamError       = "upstream_error"
	ErrCodeTimeout             = "timeout"
	ErrCodeOther               = "internal_error"
)
