package handler

import (
	"strings"
	"testing"
)

func TestSanitizeArticleHTML(t *testing.T) {
	got := sanitizeArticleHTML(`<h2>Judul</h2><p><strong>Aman</strong><img src=x onerror=alert(1)><script>alert(1)</script></p>`)
	if !strings.Contains(got, `<h2>Judul</h2><p><strong>Aman</strong></p>`) || strings.Contains(got, "alert(1)") || strings.Contains(got, "<script") || strings.Contains(got, "onerror") || strings.Contains(got, "<img") {
		t.Fatalf("hasil sanitasi tidak aman: %q", got)
	}
}
