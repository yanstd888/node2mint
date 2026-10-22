package handler

import "testing"

func TestNormalizeModelTypes(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"single", "TEXT", "TEXT", true},
		{"three unique", "text,IMAGE, reasoning", "TEXT,IMAGE,REASONING", true},
		{"empty", "", "", false},
		{"too many", "TEXT,IMAGE,VIDEO,AUDIO", "", false},
		{"duplicate", "TEXT,TEXT", "", false},
		{"unknown", "TEXT,REASONINGNESS", "", false},
		{"blank member", "TEXT,,IMAGE", "", false},
		{"embedding stays non-chat capability", "EMBEDDING", "EMBEDDING", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := normalizeModelTypes(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("normalizeModelTypes(%q)=(%q,%t), want (%q,%t)", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}
