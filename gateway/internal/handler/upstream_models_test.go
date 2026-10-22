package handler

import (
	"strings"
	"testing"
)

func TestParseUpstreamModelsDedupesIDsAndNames(t *testing.T) {
	models, err := parseUpstreamModels(strings.NewReader(`{"data":[
		{"id":"gpt-4o"}, {"id":" gpt-4o "}, {"name":"claude-sonnet"},
		{"id":""}, {}, {"id":"gpt-4o-mini","name":"ignored"}
	]}`))
	if err != nil {
		t.Fatalf("parseUpstreamModels: %v", err)
	}
	want := []string{"gpt-4o", "claude-sonnet", "gpt-4o-mini"}
	if len(models) != len(want) {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
	for i := range want {
		if models[i] != want[i] {
			t.Fatalf("models = %#v, want %#v", models, want)
		}
	}
}
