package handler

import (
	"strings"
	"testing"
)

// Guard struktural: endpoint admin fraud harus membaca kolom yang benar-benar
// ada di schema dan tetap read-only untuk daftar. Tanpa DB nyata, ini yang
// gagal kalau nama kolom/tabel bergeser — inti bug yang pernah kejadian.
func TestAdminFraudSQL(t *testing.T) {
	for _, want := range []string{"fraud_event", "is_reviewed", "is_actioned", "kind", "detail"} {
		if !strings.Contains(adminFraudListSQL, want) {
			t.Errorf("query daftar fraud tidak memuat %q", want)
		}
	}
	if !strings.Contains(adminFraudReviewSQL, "is_reviewed=true") {
		t.Errorf("query review harus menandai is_reviewed, dapat: %s", adminFraudReviewSQL)
	}
	if strings.Contains(adminFraudListSQL, "UPDATE") || strings.Contains(adminFraudListSQL, "DELETE") {
		t.Error("daftar fraud harus read-only")
	}
}
