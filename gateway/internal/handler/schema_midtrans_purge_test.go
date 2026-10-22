package handler

import (
	"os"
	"strings"
	"testing"
)

// TestSchemaMidtransPurgeSkipsFreshDatabase keeps the retired-table cleanup
// conditional: a fresh install no longer has midtrans_payment to query.
func TestSchemaMidtransPurgeSkipsFreshDatabase(t *testing.T) {
	schema, err := os.ReadFile("../../sql/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	cutover := string(schema)
	start := strings.Index(cutover, "Migrasi satu-kali: hapus total sisa Midtrans")
	if start < 0 {
		t.Fatal("blok purge Midtrans tidak ditemukan")
	}
	cutover = cutover[start:]
	guard := strings.Index(cutover, "IF to_regclass('public.midtrans_payment') IS NOT NULL THEN")
	purge := strings.Index(cutover, "DELETE FROM credit_transaction c")
	if guard < 0 || purge < 0 || guard > purge {
		t.Fatal("purge Midtrans harus memeriksa tabel legacy sebelum mereferensikannya")
	}
}
