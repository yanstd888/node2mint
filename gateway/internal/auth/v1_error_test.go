package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteV1ErrorUsesStableEnvelope(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		code     string
		wantType string
		wantCode string
	}{
		{"not found", http.StatusNotFound, "not_found", "invalid_request_error", "not_found"},
		{"method", http.StatusMethodNotAllowed, "method_not_allowed", "invalid_request_error", "method_not_allowed"},
		{"rate", http.StatusTooManyRequests, "", "rate_limit_error", "rate_limit_exceeded"},
		{"upstream", http.StatusBadGateway, "", "server_error", "upstream_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteV1Error(w, tc.status, "test message", tc.code)

			if w.Code != tc.status || w.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("status/content-type = %d/%q", w.Code, w.Header().Get("Content-Type"))
			}
			var body struct {
				Error struct {
					Message string  `json:"message"`
					Type    string  `json:"type"`
					Param   *string `json:"param"`
					Code    string  `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v (%s)", err, w.Body.String())
			}
			if body.Error.Message != "test message" || body.Error.Type != tc.wantType || body.Error.Param != nil || body.Error.Code != tc.wantCode {
				t.Fatalf("envelope=%#v", body.Error)
			}
		})
	}
}
