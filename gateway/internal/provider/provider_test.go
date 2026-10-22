package provider

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Upstream SSE: tiga delta lalu [DONE] dengan blok usage di chunk terakhir.
func TestStreamChatParsesSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("auth header salah")
		}
		var body struct {
			StreamOptions struct {
				IncludeUsage bool `json:"include_usage"`
			} `json:"stream_options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if !body.StreamOptions.IncludeUsage {
			t.Error("stream_options.include_usage harus true")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Ha\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		// 9router dapat meneruskan cache Claude sebagai field datar ini.
		w.Write([]byte("data: {\"choices\":[{\"delta\":{}}],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":2,\"total_tokens\":9,\"cache_read_input_tokens\":4}}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	p := NewOpenAI(Config{APIKey: "k", BaseURL: srv.URL + "/v1"})
	var deltas []string
	usage, err := p.StreamChat(RequestChat{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}, MaxToken: 16},
		func(d string) { deltas = append(deltas, d) })
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(deltas, ""); got != "Halo" {
		t.Fatalf("deltas = %q", got)
	}
	if usage.Content != "Halo" || usage.TotalTokens != 9 || usage.PromptTokens != 7 || usage.CompletionTokens != 2 || usage.CacheTokens != 4 {
		t.Fatalf("usage = %+v", usage)
	}
}

// Upstream non-200 sebelum byte pertama: error, bukan stream kosong.
func TestStreamChatUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()
	p := NewOpenAI(Config{BaseURL: srv.URL + "/v1"})
	if _, err := p.StreamChat(RequestChat{Model: "m"}, nil); err == nil {
		t.Fatal("error provider 502 harus dikembalikan")
	}
}

func TestStreamChatRejectsHTML200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>landing page</html>"))
	}))
	defer srv.Close()
	p := NewOpenAI(Config{BaseURL: srv.URL + "/v1"})
	if _, err := p.StreamChat(RequestChat{Model: "m"}, nil); err == nil || !strings.Contains(err.Error(), "bukan SSE") {
		t.Fatalf("HTML 200 harus ditolak, err=%v", err)
	}
}

// Routing: kolom models CSV membatasi provider; kosong = melayani semua.
func TestServesModel(t *testing.T) {
	cases := []struct {
		models, model string
		want          bool
	}{
		{"", "gpt-4o", true},
		{"gpt-4o,gpt-4o-mini", "gpt-4o-mini", true},
		{"gpt-4o, gpt-4o-mini", "gpt-4o-mini", true},
		{"gpt-4o", "claude-3-5-sonnet", false},
	}
	for _, c := range cases {
		_, got := routeModel(entry{models: c.models}, c.model)
		if got != c.want {
			t.Fatalf("routeModel(%q,%q) = %v, mau %v", c.models, c.model, got, c.want)
		}
	}
}

// Pool key: key pertama kena 429 → cooldown → request berikut langsung ke key
// kedua tanpa menyentuh key pertama lagi. Pola penampungan 9router.
func TestCooldownMelewatiKeyGagal(t *testing.T) {
	var hit1, hit2 int
	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit1++
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer fail.Close()
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit2++
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer ok.Close()

	r := &Registry{providers: []entry{
		{p: NewOpenAI(Config{APIKey: "k1", BaseURL: fail.URL + "/v1"}), name: "key-1"},
		{p: NewOpenAI(Config{APIKey: "k2", BaseURL: ok.URL + "/v1"}), name: "key-2"},
	}}
	req := RequestChat{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}}

	_, meta, err := r.ChatWithMeta(req)
	if err != nil {
		t.Fatal(err)
	}
	if hit1 != 1 || hit2 != 1 {
		t.Fatalf("setelah gagal: hit1=%d hit2=%d, mau 1/1", hit1, hit2)
	}
	if meta.Provider != "key-2" || !meta.Fallback {
		t.Fatalf("metadata failover = %+v, mau provider=key-2 fallback=true", meta)
	}
	// Request kedua: key-1 masih cooldown, tidak boleh dicoba lagi.
	if _, err := r.Chat(req); err != nil {
		t.Fatal(err)
	}
	if hit1 != 1 || hit2 != 2 {
		t.Fatalf("key-1 harus cooldown: hit1=%d hit2=%d, mau 1/2", hit1, hit2)
	}
}

// Kalau SEMUA key cooldown, tetap dicoba (paksa) — bukan gagal total.
func TestSemuaCooldownTetapDicoba(t *testing.T) {
	var hits int
	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		http.Error(w, "quota", http.StatusPaymentRequired)
	}))
	defer fail.Close()

	r := &Registry{providers: []entry{
		{p: NewOpenAI(Config{APIKey: "k1", BaseURL: fail.URL + "/v1"}), name: "satu"},
	}}
	req := RequestChat{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}}
	if _, err := r.Chat(req); err == nil {
		t.Fatal("harus gagal")
	}
	if _, err := r.Chat(req); err == nil {
		t.Fatal("harus gagal (paksa) tapi tetap dicoba")
	}
	if hits != 2 {
		t.Fatalf("hits = %d, mau 2 (cooldown tidak membuang request)", hits)
	}
}

// Durasi cooldown per jenis galat.
func TestCooldownFrom(t *testing.T) {
	cases := []struct {
		err  string
		want time.Duration
	}{
		{"provider 429: rate limited", time.Minute},
		{"provider 402: quota", 10 * time.Minute},
		{"provider 500: upstream", 30 * time.Second},
		{"provider 400: bad request", 30 * time.Second},
		{"provider request failed: dial tcp refused", 30 * time.Second},
	}
	for _, c := range cases {
		if got := cooldownFrom(errors.New(c.err)); got != c.want {
			t.Fatalf("cooldownFrom(%q) = %s, mau %s", c.err, got, c.want)
		}
	}
}
