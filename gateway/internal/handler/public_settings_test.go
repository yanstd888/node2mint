package handler

import "testing"

func TestPublicHTTPURL(t *testing.T) {
	for _, c := range []struct {
		in, want string
	}{
		{"https://discord.gg/aisell", "https://discord.gg/aisell"},
		{"http://localhost:3000/community", "http://localhost:3000/community"},
		{"javascript:alert(1)", ""},
		{"data:text/html,hello", ""},
		{"discord.gg/aisell", ""},
	} {
		if got := publicHTTPURL(c.in); got != c.want {
			t.Fatalf("publicHTTPURL(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestPublicAssetURL(t *testing.T) {
	for input, want := range map[string]string{
		"/api/media/banner":          "/api/media/banner",
		"https://example.com/x.webp": "https://example.com/x.webp",
		"javascript:alert(1)":        "",
		"//evil.example/x":           "",
	} {
		if got := publicAssetURL(input); got != want {
			t.Fatalf("publicAssetURL(%q) = %q; want %q", input, got, want)
		}
	}
}
