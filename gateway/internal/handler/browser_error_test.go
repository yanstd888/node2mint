package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFailUsesStableBrowserError(t *testing.T) {
	w := httptest.NewRecorder()
	fail(w, http.StatusTooManyRequests, "too many requests")

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if w.Code != http.StatusTooManyRequests || body.Error.Code != "rate_limit_exceeded" || body.Error.Message != "too many requests" {
		t.Fatalf("status/error=%d/%#v", w.Code, body.Error)
	}
}

func TestFailCodePreservesSemanticCode(t *testing.T) {
	w := httptest.NewRecorder()
	failCode(w, http.StatusUnauthorized, "two_factor_required", "kode 2FA diperlukan")

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "two_factor_required" {
		t.Fatalf("code=%q", body.Error.Code)
	}
}
