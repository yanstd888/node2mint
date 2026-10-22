package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Cek yang gagal kalau body ping kembali ke max_tokens kecil: router nyata
// (mis. vyceai.com/v1) menjawab 400 invalid_request untuk max_tokens=1, jadi
// tombol Check melaporkan provider sehat sebagai galat. Lihat PingProviderBody.
func TestPingProviderBodyMaxTokensDiAtasMinimumRouter(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		if got := r.Header.Get("Authorization"); got != "Bearer sk-uji" {
			t.Errorf("Authorization = %q, mau \"Bearer sk-uji\"", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	code, healthy, detail := pingProvider(srv.URL, "sk-uji", "gpt-4o")
	if code != 200 || !healthy || detail != "" {
		t.Fatalf("code=%d healthy=%v detail=%q, mau 200/true/\"\"", code, healthy, detail)
	}
	mt, _ := body["max_tokens"].(float64)
	if mt < 16 {
		t.Fatalf("max_tokens = %v, mau >= 16 (router menolak nilai lebih kecil)", body["max_tokens"])
	}
	if body["model"] != "gpt-4o" {
		t.Fatalf("model = %v, mau gpt-4o", body["model"])
	}
}

// listProviderModels: baca {base}/models. Router yang membalas bukan bentuk
// OpenAI (atau tanpa data) → nil, bukan galat: ping tetap penentu utama.
func TestListProviderModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-uji" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"claude-sonnet-4-6"},{"id":"deepseek-v4.1"}]}`)
	}))
	defer srv.Close()

	got := listProviderModels(srv.URL+"/v1", "sk-uji")
	if len(got) != 2 || got[0] != "claude-sonnet-4-6" || got[1] != "deepseek-v4.1" {
		t.Fatalf("models = %v", got)
	}
	if bad := listProviderModels(srv.URL+"/v1", "sk-salah"); bad != nil {
		t.Fatalf("key salah harus nil, dapat %v", bad)
	}
}

func TestProviderCheckModelUsesConfiguredCSV(t *testing.T) {
	if got := providerCheckModel("", "claude-sonnet-4-6, deepseek-v4.1"); got != "claude-sonnet-4-6" {
		t.Fatalf("model=%q, mau model pertama dari provider.models", got)
	}
	if got := providerCheckModel("model-khusus", "claude-sonnet-4-6"); got != "model-khusus" {
		t.Fatalf("model_id eksplisit berubah menjadi %q", got)
	}
}

func TestProviderTestModelFallsBackToDiscoveredModel(t *testing.T) {
	got, replaced := providerTestModel("gpt-4o", []string{"claude-sonnet-4-6", "deepseek-v4.1"})
	if got != "claude-sonnet-4-6" || !replaced {
		t.Fatalf("model=%q replaced=%v, mau model pertama upstream dan replaced=true", got, replaced)
	}

	got, replaced = providerTestModel("deepseek-v4.1", []string{"claude-sonnet-4-6", "deepseek-v4.1"})
	if got != "deepseek-v4.1" || replaced {
		t.Fatalf("model valid berubah: model=%q replaced=%v", got, replaced)
	}
}

// Galat provider harus dibalikkan apa adanya supaya UI bisa menampilkan pesannya.
func TestPingProviderMengembalikanBodyGalat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer srv.Close()

	code, healthy, detail := pingProvider(srv.URL, "sk-x", "gpt-4o")
	if code != 401 || healthy {
		t.Fatalf("code=%d healthy=%v, mau 401/false", code, healthy)
	}
	if detail != `{"error":{"message":"invalid api key"}}` {
		t.Fatalf("detail = %q", detail)
	}
}
