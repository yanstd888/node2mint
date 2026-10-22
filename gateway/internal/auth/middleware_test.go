package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ai-sell/gateway/internal/apilog"
)

// Cookie sesi harus diterima setara Bearer (web memakai cookie, API client
// memakai header), dan Bearer menang bila keduanya ada.
func TestRawToken(t *testing.T) {
	for _, tc := range []struct {
		name   string
		auth   string
		cookie string
		want   string
	}{
		{"bearer saja", "Bearer tok-a", "", "tok-a"},
		{"cookie saja", "", "tok-b", "tok-b"},
		{"bearer menang", "Bearer tok-a", "tok-b", "tok-a"},
		{"tanpa keduanya", "", "", ""},
		{"skema lain diabaikan", "Basic abc", "tok-b", "tok-b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
			if tc.auth != "" {
				r.Header.Set("Authorization", tc.auth)
			}
			if tc.cookie != "" {
				r.AddCookie(&http.Cookie{Name: CookieName, Value: tc.cookie})
			}
			if got := RawToken(r); got != tc.want {
				t.Fatalf("RawToken = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRequireAPIKeyMissingKeyWritesJSON(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	RequireAPIKey(nil)(next).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var body struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not JSON: %v (%q)", err, w.Body.String())
	}
	if body.Error.Message != "missing api key" || body.Error.Type != "authentication_error" || body.Error.Code != "missing_api_key" {
		t.Fatalf("error envelope = %#v", body.Error)
	}
}

func TestRequireAdminRejectsNonAdmin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, tc := range []struct {
		name string
		role string
		want int
	}{
		{"missing claims", "", http.StatusForbidden},
		{"user role", "USER", http.StatusForbidden},
		{"admin role", "ADMIN", http.StatusNoContent},
		{"super admin role", "SUPER_ADMIN", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/admin/providers/id/models/import", nil)
			if tc.role != "" {
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, &Claims{Role: tc.role}))
			}
			w := httptest.NewRecorder()
			RequireAdmin(next).ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}

func TestRequireScope(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, tc := range []struct {
		name, scopes string
		want         int
	}{
		{"allows exact scope", "chat", http.StatusNoContent},
		{"allows CSV scope", "read, chat,write", http.StatusNoContent},
		{"rejects missing scope", "read,write", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, meta := apilog.With(context.Background())
			ctx = context.WithValue(ctx, apiKeyScopesKey{}, tc.scopes)
			r := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil).WithContext(ctx)
			w := httptest.NewRecorder()
			RequireScope("chat")(next).ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
			if tc.want != http.StatusForbidden {
				return
			}
			if meta.ErrorCode != "permission_denied" {
				t.Fatalf("meta.ErrorCode = %q, want permission_denied", meta.ErrorCode)
			}
			if got := w.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}
			var body struct {
				Error struct {
					Message string `json:"message"`
					Type    string `json:"type"`
					Code    string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("error body is not JSON: %v (%q)", err, w.Body.String())
			}
			if body.Error.Type != "permission_error" || body.Error.Code != "permission_denied" {
				t.Fatalf("error envelope = %#v", body.Error)
			}
		})
	}
}
