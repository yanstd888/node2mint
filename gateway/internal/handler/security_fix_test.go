package handler

import (
	"net/http/httptest"
	"testing"
)

// Satu gerbang NaN: "NaN"/"Infinity"/"1e999" harus ditolak di sini, karena
// NaN membuat setiap perbandingan rentang bernilai false dan lolos ke NUMERIC.
// Float() tidak punya fallback (selalu 0 saat tidak sah); Int() punya.
func TestNumRejectsNaN(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"NaN", 0},
		{"nan", 0},
		{"Infinity", 0},
		{"-Inf", 0},
		{"1e999", 0},
		{"", 0},
		{"abc", 0},
		{"-5", -5},
		{"12500.5", 12500.5},
	}
	for _, c := range cases {
		if got := num(c.in).Float(); got != c.want {
			t.Errorf("num(%q).Float() = %v, mau %v", c.in, got, c.want)
		}
	}
	if got := num("NaN").Int(9); got != 9 {
		t.Errorf("num(NaN).Int(9) = %d, mau 9", got)
	}
	if got := num("42").Int(1); got != 42 {
		t.Errorf("num(42).Int(1) = %d, mau 42", got)
	}
}

// Monitor uptime: host non-publik ditolak (SSRF), dengan katup dev eksplisit.
func TestUptimeHostAllowed(t *testing.T) {
	blocked := []string{"localhost", "127.0.0.1", "10.1.2.3", "172.18.0.1", "192.168.1.9", "169.254.169.254", "[::1]", "gateway", ""}
	for _, h := range blocked {
		if uptimeHostAllowed(h, false) {
			t.Errorf("host %q seharusnya ditolak", h)
		}
	}
	for _, h := range []string{"example.com", "api.openai.com", "8.8.8.8", "127.0.0.1"} {
		if !uptimeHostAllowed(h, true) {
			t.Errorf("host %q seharusnya lolos saat allowInternal", h)
		}
	}
	if !uptimeHostAllowed("example.com", false) {
		t.Error("host publik seharusnya lolos")
	}
}

// M-2: dial-time guard harus menolak alamat IP internal walau namanya "publik".
func TestSafeDialControl(t *testing.T) {
	blokir := []string{"127.0.0.1:80", "10.1.2.3:8080", "192.168.1.9:443", "169.254.169.254:80", "[::1]:80", "0.0.0.0:80"}
	for _, addr := range blokir {
		if err := safeDialControl(false)("tcp", addr, nil); err == nil {
			t.Errorf("dial %q seharusnya ditolak", addr)
		}
	}
	if err := safeDialControl(false)("tcp", "93.184.216.34:80", nil); err != nil {
		t.Errorf("alamat publik ditolak: %v", err)
	}
	// Katup dev: monitor lokal memang ke 127.0.0.1 di dev.
	if err := safeDialControl(true)("tcp", "127.0.0.1:80", nil); err != nil {
		t.Errorf("allowInternal seharusnya meloloskan loopback: %v", err)
	}
}

// L-4: kuota media diuji lewat batasnya, bukan dengan mengunggah 200 berkas.
func TestMediaQuotaExceeded(t *testing.T) {
	cases := []struct {
		name  string
		files int64
		used  int64
		next  int
		want  bool
	}{
		{"akun baru lolos", 0, 0, 4 << 20, false},
		{"berkas ke-200 masih lolos", maxMediaFiles - 1, 0, 1024, false},
		{"berkas ke-201 ditolak", maxMediaFiles, 0, 1, true},
		{"total pas batas masih lolos", 1, maxMediaTotalBytes - 1024, 1024, false},
		{"total lewat batas ditolak", 1, maxMediaTotalBytes - 1023, 1024, true},
	}
	for _, c := range cases {
		if got := MediaQuotaExceeded(c.files, c.used, c.next); got != c.want {
			t.Errorf("%s: MediaQuotaExceeded(%d,%d,%d) = %v, mau %v", c.name, c.files, c.used, c.next, got, c.want)
		}
	}
}

func TestRealIPTrustsOnlyInternalProxy(t *testing.T) {
	internal := httptest.NewRequest("GET", "/", nil)
	internal.RemoteAddr = "172.18.0.2:1234"
	internal.Header.Set("X-Real-IP", "203.0.113.9")
	if got := RealIP(internal); got != "203.0.113.9" {
		t.Fatalf("internal proxy RealIP = %q", got)
	}

	public := httptest.NewRequest("GET", "/", nil)
	public.RemoteAddr = "198.51.100.8:1234"
	public.Header.Set("X-Real-IP", "203.0.113.9")
	if got := RealIP(public); got != "198.51.100.8" {
		t.Fatalf("public client must not control RealIP, got %q", got)
	}
}

func TestProductDescriptionLimits(t *testing.T) {
	base := productInput{Title: "Produk uji", Price: num("1000"), Stock: num("1")}
	base.Description = "terlalu pendek"
	if _, _, msg := base.validate(); msg != "deskripsi minimal 20 karakter" {
		t.Fatalf("deskripsi pendek: %q", msg)
	}
	base.Description = "Deskripsi produk yang cukup lengkap."
	if _, _, msg := base.validate(); msg != "" {
		t.Fatalf("deskripsi sah ditolak: %q", msg)
	}
	if maxMediaBytes != 1<<20 {
		t.Fatalf("batas media = %d, mau 1 MB", maxMediaBytes)
	}
}

func TestURLHost(t *testing.T) {
	if got := urlHost("http://gateway:8080/healthz"); got != "gateway" {
		t.Errorf("urlHost = %q, mau gateway", got)
	}
	if got := urlHost("https://example.com/x?y=1"); got != "example.com" {
		t.Errorf("urlHost = %q, mau example.com", got)
	}
}
