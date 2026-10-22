package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteBrowserErrorUsesStableEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	WriteBrowserError(w, "akses ditolak", http.StatusForbidden)

	if w.Code != http.StatusForbidden || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status/content-type = %d/%q", w.Code, w.Header().Get("Content-Type"))
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "forbidden" || body.Error.Message != "akses ditolak" {
		t.Fatalf("error=%#v", body.Error)
	}
}
