package handler

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os/exec"
	"testing"
)

// TestConvertToWebPSucceeds: setiap gambar yang masuk harus keluar sebagai
// WebP asli (magic "RIFF"...."WEBP"), bukan sekadar nama berkas baru. Dilewati
// bila cwebp tidak ada di PATH (mis. mesin dev tanpa libwebp-tools).
func TestConvertToWebPSucceeds(t *testing.T) {
	if _, err := exec.LookPath("cwebp"); err != nil {
		t.Skip("cwebp tidak ada di PATH — lewati uji konversi WebP")
	}
	src := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			src.Set(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 30), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	out, err := convertToWebP(buf.Bytes(), false)
	if err != nil {
		t.Fatalf("convertToWebP: %v", err)
	}
	if len(out) < 12 || string(out[0:4]) != "RIFF" || string(out[8:12]) != "WEBP" {
		t.Fatalf("keluaran bukan WebP: %q", out[:min(12, len(out))])
	}
	if _, err := convertToWebP([]byte("bukan gambar"), false); err == nil {
		t.Fatal("input non-gambar seharusnya gagal, bukan lolos")
	}
}
