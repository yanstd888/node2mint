package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Cek jalur "down": server balas 500 → status down + pesan memuat kode HTTP.
func TestProbeReportsDownOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	status, msg, _ := probe(&http.Client{Timeout: 2 * time.Second}, srv.URL)
	if status != "down" {
		t.Fatalf("status = %q, mau down", status)
	}
	if msg != "HTTP 500" {
		t.Fatalf("msg = %q, mau \"HTTP 500\"", msg)
	}
}

// Cek jalur bahagia: 200 → up + latensi tercatat.
func TestProbeReportsUpOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	status, msg, ping := probe(&http.Client{Timeout: 2 * time.Second}, srv.URL)
	if status != "up" || msg != "OK" || ping < 0 {
		t.Fatalf("got status=%q msg=%q ping=%d, mau up/OK/>=0", status, msg, ping)
	}
}

// URL push Kuma harus berbentuk /api/push/<token>?msg=<encoded>&ping=N&status=up
func TestPushQueryShape(t *testing.T) {
	q := pushQuery("up", "bad gateway error", 12)
	if q.Get("status") != "up" || q.Get("ping") != "12" || q.Get("msg") != "bad gateway error" {
		t.Fatalf("query salah: %s", q.Encode())
	}
}

func TestUptimeDownNotification(t *testing.T) {
	title, message := uptimeDownNotification(uptimeResult{Name: "Node2Mint API models"})
	if title != "Uptime monitor down" || message != "Node2Mint API models gagal 3 kali berturut-turut." {
		t.Fatalf("unexpected uptime alert: title=%q message=%q", title, message)
	}
}
