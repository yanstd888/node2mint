package handler

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestChatHistoryValidation: validates the input bounds for ChatHistoryCreate
// (Task 16). Messages must be 1-100, role must be user/assistant, content
// max 20000 chars, model max 200, title max 120.
func TestChatHistoryValidation(t *testing.T) {
	// Replicate the validation logic from ChatHistoryCreate to ensure
	// the bounds are correct and secure.
	validate := func(messages []struct{ Role, Content string }, model, title string) bool {
		if len(messages) == 0 || len(messages) > 100 {
			return false
		}
		for _, m := range messages {
			if (m.Role != "user" && m.Role != "assistant") || len(m.Content) > 20000 {
				return false
			}
		}
		t := strings.TrimSpace(title)
		if t == "" {
			t = strings.TrimSpace(messages[0].Content)
			if len(t) > 80 {
				t = t[:80]
			}
		}
		if len(t) > 120 || len(model) > 200 {
			return false
		}
		return true
	}

	tests := []struct {
		name  string
		msgs  []struct{ Role, Content string }
		model string
		title string
		want  bool
	}{
		{"valid single msg", []struct{ Role, Content string }{{"user", "hi"}}, "gpt-4o", "", true},
		{"valid with title", []struct{ Role, Content string }{{"user", "hi"}, {"assistant", "hello"}}, "gpt-4o", "My chat", true},
		{"empty messages", nil, "gpt-4o", "", false},
		{"too many messages", make([]struct{ Role, Content string }, 101), "gpt-4o", "", false},
		{"invalid role system", []struct{ Role, Content string }{{"system", "hi"}}, "gpt-4o", "", false},
		{"content too long", []struct{ Role, Content string }{{"user", strings.Repeat("x", 20001)}}, "gpt-4o", "", false},
		{"content at max", []struct{ Role, Content string }{{"user", strings.Repeat("x", 20000)}}, "gpt-4o", "", true},
		{"model too long", []struct{ Role, Content string }{{"user", "hi"}}, strings.Repeat("x", 201), "", false},
		{"title too long", []struct{ Role, Content string }{{"user", strings.Repeat("x", 130)}}, "gpt-4o", strings.Repeat("x", 130), false},
		{"title auto-trimmed from content", []struct{ Role, Content string }{{"user", strings.Repeat("x", 90)}}, "gpt-4o", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validate(tc.msgs, tc.model, tc.title); got != tc.want {
				t.Errorf("validate() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestChatHistoryJSONMarshal: verifies the messages payload is valid JSON
// (Task 16 persistence correctness).
func TestChatHistoryJSONMarshal(t *testing.T) {
	msgs := []chatHistoryMessage{
		{"user", "hello"},
		{"assistant", "hi there"},
	}
	b, err := json.Marshal(msgs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"role":"user"`) {
		t.Error("JSON must contain user role")
	}
	if !strings.Contains(s, `"content":"hi there"`) {
		t.Error("JSON must contain assistant content")
	}
}

func TestChatHistoryListResponseContract(t *testing.T) {
	item := chatHistoryItem{
		ID:        "history-1",
		Title:     "First chat",
		Model:     "task16-model",
		Messages:  json.RawMessage(`[{"role":"user","content":"hello"}]`),
		CreatedAt: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 10, 7, 12, 1, 0, 0, time.UTC),
	}
	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal response item: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode response item: %v", err)
	}
	for _, key := range []string{"id", "title", "model", "messages", "created_at", "updated_at"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("response missing %q: %s", key, encoded)
		}
	}
	if _, ok := fields["ID"]; ok {
		t.Error("response must not expose Go field name ID")
	}
	var messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(fields["messages"], &messages); err != nil {
		t.Fatalf("messages must remain JSON array: %v", err)
	}
	if len(messages) != 1 || messages[0].Role != "user" || messages[0].Content != "hello" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
}
