package handler

import "testing"

func TestDetectLinksBlocksExecutableURIs(t *testing.T) {
	for _, input := range []string{
		"javascript:alert(1)",
		"DATA:text/html;base64,PHNjcmlwdD4=",
		"vbscript:msgbox(1)",
	} {
		if got := detectLinks(input); len(got) == 0 {
			t.Fatalf("detectLinks(%q) allowed an executable URI", input)
		}
	}
}
