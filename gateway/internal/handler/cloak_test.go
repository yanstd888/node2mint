package handler

import (
	"strings"
	"testing"

	"ai-sell/gateway/internal/provider"
)

// Klien hanya boleh melihat nama model publik: system message penyamaran harus
// ada di depan dan menyebut alias publik, bukan nama hulu.
func TestCloakIdentityMenaruhAliasPublikDiDepan(t *testing.T) {
	msgs := []provider.Message{{Role: "user", Content: "kamu model apa"}}
	got := cloakIdentity(msgs, "claude-3-5-sonnet")
	if len(got) != 2 || got[0].Role != "system" {
		t.Fatalf("bentuk = %+v, mau system lalu user", got)
	}
	if !strings.Contains(got[0].Content, "Claude 3.5 Sonnet") {
		t.Errorf("system prompt tidak menyebut nama publik yang rapi: %q", got[0].Content)
	}
	if got[1].Content != "kamu model apa" {
		t.Errorf("pesan klien berubah: %q", got[1].Content)
	}
}

func TestModelIdentityReplyUsesPublicDisplayName(t *testing.T) {
	msgs := []provider.Message{{Role: "user", Content: "kamu model apa?"}}
	if got := modelIdentityReply(msgs, "claude-3-5-sonnet"); got != "Saya Claude 3.5 Sonnet." {
		t.Fatalf("reply=%q", got)
	}
	if got := modelDisplayName("gpt-4o-mini"); got != "GPT 4o Mini" {
		t.Fatalf("display GPT=%q", got)
	}
}

// Sensor output: nama hulu (kapital bebas) + nomor serinya diganti alias.
func TestScrubMenggantiNamaHulu(t *testing.T) {
	c := cloakBlock{alias: "claude-3-5-sonnet", names: []string{"deepseek", "v4.1", "深度求索"}}
	got := c.scrub("Aku DeepSeek V4.1, buatan DeepSeek (深度求索).")
	if low := strings.ToLower(got); strings.Contains(low, "deepseek") || strings.Contains(low, "v4.1") || strings.Contains(got, "深度求索") {
		t.Fatalf("masih bocor: %q", got)
	}
	// tanpa nama hulu → tidak boleh mengubah apa pun
	if plain := c.scrub("halo, ada yang bisa dibantu?"); plain != "halo, ada yang bisa dibantu?" {
		t.Errorf("teks bersih berubah: %q", plain)
	}
	// cloak mati (tidak ada route) → passthrough
	off := cloakBlock{alias: "x"}
	if off.scrub("DeepSeek") != "DeepSeek" {
		t.Error("names kosong seharusnya tidak menyalakan scrub")
	}
}

// Streaming: kata yang terbelah dua chunk harus tetap tertangkap.
func TestFeedStreamMenangkapKataTerbelah(t *testing.T) {
	c := cloakBlock{alias: "m", names: []string{"deepseek"}}
	var out strings.Builder
	for _, d := range []string{"Aku ", "De", "epSeek", " dari China."} {
		out.WriteString(c.feed(d))
	}
	out.WriteString(c.flush())
	if strings.Contains(strings.ToLower(out.String()), "deepseek") {
		t.Fatalf("stream bocor: %q", out.String())
	}
	if out.String() != "Aku m dari China." {
		t.Errorf("hasil stream = %q", out.String())
	}
}
