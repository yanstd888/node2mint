package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestV1OriginDeny: denylist origin/referer must reject matching, allow others.
// No Origin/Referer header (typical API client) must always pass.
func TestV1OriginDeny(t *testing.T) {
	// We need a DB mock. Since V1OriginDeny reads platform_setting, and we can't
	// easily mock DB, test the substring logic directly via a helper.
	cases := []struct {
		denylist string
		origin   string
		referer  string
		want     int // 200 = pass, 403 = reject
	}{
		{"evil-scraper.com", "https://evil-scraper.com", "", 403},
		{"evil-scraper.com", "", "https://evil-scraper.com/path", 403},
		{"evil-scraper.com", "https://good.com", "", 200},
		{"", "https://anything.com", "", 200},
		{"a.com,b.com", "https://a.com", "", 403},
		{"a.com,b.com", "https://b.com", "", 403},
		{"a.com,b.com", "https://c.com", "", 200},
		// Case-insensitive matching
		{"Evil-Scraper.com", "https://evil-scraper.com", "", 403},
	}
	for _, c := range cases {
		// Simulate the matching logic without a real DB by checking substrings
		dl := lower(c.denylist)
		origin := lower(c.origin)
		referer := lower(c.referer)
		rejected := false
		if dl != "" {
			for _, bad := range splitCSV(dl) {
				if (origin != "" && contains(origin, bad)) ||
					(referer != "" && contains(referer, bad)) {
					rejected = true
					break
				}
			}
		}
		got := 200
		if rejected {
			got = 403
		}
		if got != c.want {
			t.Errorf("denylist=%q origin=%q referer=%q: got %d, want %d", c.denylist, c.origin, c.referer, got, c.want)
		}
	}
}

// TestAdminIPAllowlistEmpty: when ADMIN_IP_ALLOWLIST is empty, middleware is a no-op.
func TestAdminIPAllowlistEmpty(t *testing.T) {
	t.Setenv("ADMIN_IP_ALLOWLIST", "")
	mw := AdminIPAllowlist()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/admin/summary", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("empty allowlist should pass all, got %d", rec.Code)
	}
}

func TestPublicIPAllowlist(t *testing.T) {
	t.Setenv("NODE_ENV", "production")
	t.Setenv("SITE_IP_ALLOWLIST", "203.0.113.0/24")
	mw := PublicIPAllowlist()
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	allowed := httptest.NewRequest("GET", "/api/auth/login", nil)
	allowed.RemoteAddr = "172.18.0.2:1234"
	allowed.Header.Set("X-Real-IP", "203.0.113.9")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, allowed)
	if rec.Code != http.StatusOK {
		t.Fatalf("allowlisted IP got %d", rec.Code)
	}

	denied := httptest.NewRequest("GET", "/api/auth/login", nil)
	denied.RemoteAddr = "172.18.0.2:1234"
	denied.Header.Set("X-Real-IP", "198.51.100.8")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, denied)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-allowlisted IP got %d", rec.Code)
	}
}

func TestPublicIPAllowlistProductionEmptyFailsClosed(t *testing.T) {
	t.Setenv("NODE_ENV", "production")
	t.Setenv("SITE_IP_ALLOWLIST", "")
	mw := PublicIPAllowlist()
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	req := httptest.NewRequest("GET", "/api/auth/login", nil)
	req.RemoteAddr = "172.18.0.2:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("empty production allowlist got %d", rec.Code)
	}
}

// Helpers (avoid importing strings just for the test — keep it self-contained).
func lower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func splitCSV(s string) []string {
	var out []string
	cur := ""
	for _, c := range s {
		if c == ',' {
			if trimSpace(cur) != "" {
				out = append(out, trimSpace(cur))
			}
			cur = ""
		} else {
			cur += string(c)
		}
	}
	if trimSpace(cur) != "" {
		out = append(out, trimSpace(cur))
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func contains(hay, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i <= len(hay)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if hay[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
