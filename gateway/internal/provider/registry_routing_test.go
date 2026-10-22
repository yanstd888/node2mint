package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteModelUsesExplicitAlias(t *testing.T) {
	e := entry{models: "legacy-model", routes: map[string]string{"claude-3-5-sonnet": "upstream-sonnet-v2"}}
	got, ok := routeModel(e, "claude-3-5-sonnet")
	if !ok || got != "upstream-sonnet-v2" {
		t.Fatalf("routeModel() = %q, %v; want upstream-sonnet-v2, true", got, ok)
	}
	got, ok = routeModel(e, "legacy-model")
	if !ok || got != "legacy-model" {
		t.Fatal("route eksplisit model lain tidak boleh mematikan routing CSV legacy")
	}

	other := entry{models: "claude-3-5-sonnet", routes: map[string]string{"claude-3-5-sonnet": ""}}
	if _, ok := routeModel(other, "claude-3-5-sonnet"); ok {
		t.Fatal("provider lain tidak boleh melayani alias yang dipatok eksplisit")
	}
}

func TestRouteModelKeepsLegacySameNameRouting(t *testing.T) {
	e := entry{models: "gpt-4o, gpt-4o-mini"}
	got, ok := routeModel(e, "gpt-4o-mini")
	if !ok || got != "gpt-4o-mini" {
		t.Fatalf("routeModel() = %q, %v; want gpt-4o-mini, true", got, ok)
	}
}

func TestChatSendsMappedModelToUpstream(t *testing.T) {
	var received string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		received = body.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	r := &Registry{providers: []entry{{
		p:      NewOpenAI(Config{APIKey: "test-key", BaseURL: srv.URL + "/v1"}),
		name:   "test",
		routes: map[string]string{"claude-3-5-sonnet": "upstream-sonnet-v2"},
	}}}
	if _, err := r.Chat(RequestChat{Model: "claude-3-5-sonnet"}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if received != "upstream-sonnet-v2" {
		t.Fatalf("upstream menerima model %q, mau upstream-sonnet-v2", received)
	}
}

func TestMergeRouteKeepsBothMappedProviders(t *testing.T) {
	const alias = "deepseek-v-4-1-flash"
	provs := []entry{{id: "private", routes: map[string]string{}}, {id: "vyceai", routes: map[string]string{}}}
	byID := map[string][]int{"private": {0}, "vyceai": {1}}
	initialized := map[string]struct{}{}

	mergeRoute(provs, byID, initialized, "private", alias, "CODE-V.2-YANSTD")
	mergeRoute(provs, byID, initialized, "vyceai", alias, "deepseek-v4.1")

	if got := provs[0].routes[alias]; got != "CODE-V.2-YANSTD" {
		t.Fatalf("primary route = %q, want CODE-V.2-YANSTD", got)
	}
	if got := provs[1].routes[alias]; got != "deepseek-v4.1" {
		t.Fatalf("secondary route = %q, want deepseek-v4.1", got)
	}
}

func TestChatFailsOverBetweenExplicitRoutes(t *testing.T) {
	const alias = "deepseek-v-4-1-flash"
	var primaryHits, secondaryHits int
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode primary request: %v", err)
		}
		if body.Model != "CODE-V.2-YANSTD" {
			t.Fatalf("primary model = %q", body.Model)
		}
		primaryHits++
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer primary.Close()
	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode secondary request: %v", err)
		}
		if body.Model != "deepseek-v4.1" {
			t.Fatalf("secondary model = %q", body.Model)
		}
		secondaryHits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer secondary.Close()

	r := &Registry{providers: []entry{
		{p: NewOpenAI(Config{APIKey: "test", BaseURL: primary.URL + "/v1"}), name: "private", routes: map[string]string{alias: "CODE-V.2-YANSTD"}},
		{p: NewOpenAI(Config{APIKey: "test", BaseURL: secondary.URL + "/v1"}), name: "vyceai", routes: map[string]string{alias: "deepseek-v4.1"}},
	}}
	usage, meta, err := r.ChatWithMeta(RequestChat{Model: alias, Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("ChatWithMeta: %v", err)
	}
	if primaryHits != 1 || secondaryHits != 1 {
		t.Fatalf("hits primary/secondary = %d/%d, want 1/1", primaryHits, secondaryHits)
	}
	if usage.Content != "ok" || meta.Provider != "vyceai" || !meta.Fallback {
		t.Fatalf("usage/meta = %#v/%#v", usage, meta)
	}
}
