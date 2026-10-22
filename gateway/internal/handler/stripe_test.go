package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

// Satu cek jalur uang Stripe: tanda tangan sah diterima, payload diubah dan
// replay >5 menit ditolak. Murni HMAC — tanpa koneksi ke Stripe.
func TestStripeSignature(t *testing.T) {
	secret := "whsec_test"
	body := []byte(`{"type":"checkout.session.completed"}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	header := "t=" + ts + ",v1=" + sig

	if !verifyStripeSignature(secret, header, body) {
		t.Fatal("tanda tangan sah ditolak")
	}
	if verifyStripeSignature(secret, header, []byte(`{"type":"tampered"}`)) {
		t.Fatal("payload diubah diterima")
	}
	old := "t=" + strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10) + ",v1=" + sig
	if verifyStripeSignature(secret, old, body) {
		t.Fatal("replay >5 menit diterima")
	}
	if verifyStripeSignature(secret, "t="+ts+",v1=deadbeef", body) {
		t.Fatal("tanda tangan ngawur diterima")
	}
}
