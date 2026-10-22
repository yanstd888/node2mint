package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Uji parsing respons Responses API (Chat + StreamChat) tanpa jaringan.
func TestResponsesParse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("path = %s, mau /responses", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer k3y" {
			t.Errorf("auth header hilang")
		}
		var streaming bool
		if err := json.NewDecoder(r.Body).Decode(&struct {
			Stream *bool `json:"stream"`
		}{Stream: &streaming}); err != nil {
			t.Fatalf("body: %v", err)
		}
		if !streaming {
			// Non-stream: Responses API membalas JSON, bukan SSE.
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"Halo"}]}],"usage":{"input_tokens":3,"output_tokens":5,"total_tokens":8}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		for _, ev := range []string{
			`event: response.output_text.delta` + "\n" + `data: {"type":"response.output_text.delta","delta":"Ha"}`,
			`event: response.output_text.delta` + "\n" + `data: {"type":"response.output_text.delta","delta":"lo"}`,
			`event: response.completed` + "\n" + `data: {"type":"response.completed","response":{"usage":{"input_tokens":3,"output_tokens":5,"total_tokens":8}}}`,
		} {
			w.Write([]byte(ev + "\n\n"))
			w.(http.Flusher).Flush()
		}
	}))
	defer srv.Close()

	p := NewResponses(Config{APIKey: "k3y", BaseURL: srv.URL})
	got, err := p.Chat(RequestChat{Model: "m-1", Messages: []Message{{Role: "user", Content: "hai"}}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got.Content != "Halo" {
		t.Errorf("Content = %q, mau %q", got.Content, "Halo")
	}
	if got.PromptTokens != 3 || got.CompletionTokens != 5 || got.TotalTokens != 8 {
		t.Errorf("usage = %d/%d/%d, mau 3/5/8", got.PromptTokens, got.CompletionTokens, got.TotalTokens)
	}

	var sb strings.Builder
	_, err = p.StreamChat(RequestChat{Model: "m-1", Messages: []Message{{Role: "user", Content: "hai"}}}, func(d string) { sb.WriteString(d) })
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}
	if sb.String() != "Halo" {
		t.Errorf("stream = %q, mau %q", sb.String(), "Halo")
	}
}
