package auth

import (
	"encoding/json"
	"net/http"
)

// WriteV1Error keeps programmatic API failures machine-readable for /v1
// responses. Browser-facing /api routes use WriteBrowserError below.
func WriteV1Error(w http.ResponseWriter, status int, message, code string) {
	if code == "" {
		code = v1ErrorCode(status)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
		"message": message,
		"type":    v1ErrorType(status),
		"param":   nil,
		"code":    code,
	}})
}

// WriteBrowserError returns the stable JSON envelope used by browser-facing APIs.
// The message remains useful for logs/debugging; clients render the code locally.
func WriteBrowserError(w http.ResponseWriter, message string, status int) {
	WriteBrowserErrorCode(w, message, status, "")
}

func WriteBrowserErrorCode(w http.ResponseWriter, message string, status int, code string) {
	if code == "" {
		code = browserErrorCode(status)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
		"message": message,
		"code":    code,
	}})
}

func browserErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusPaymentRequired:
		return "payment_required"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case http.StatusUnprocessableEntity:
		return "validation_failed"
	case http.StatusTooManyRequests:
		return "rate_limit_exceeded"
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return "upstream_error"
	case http.StatusInternalServerError:
		return "internal_error"
	default:
		return "request_failed"
	}
}
func v1ErrorType(status int) string {
	switch {
	case status == http.StatusUnauthorized:
		return "authentication_error"
	case status == http.StatusForbidden:
		return "permission_error"
	case status == http.StatusTooManyRequests:
		return "rate_limit_error"
	case status >= 500:
		return "server_error"
	default:
		return "invalid_request_error"
	}
}

func v1ErrorCode(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "invalid_api_key"
	case http.StatusPaymentRequired:
		return "insufficient_quota"
	case http.StatusForbidden:
		return "permission_denied"
	case http.StatusTooManyRequests:
		return "rate_limit_exceeded"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return "upstream_error"
	case http.StatusInternalServerError:
		return "server_error"
	default:
		return "invalid_request_error"
	}
}
