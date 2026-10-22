package handler

import (
	"os"
	"strings"
	"testing"
)

func TestTokenPackageSchemaIncludesAdminListColumns(t *testing.T) {
	schema, err := os.ReadFile("../../sql/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(schema), "ALTER TABLE token_package ADD COLUMN IF NOT EXISTS sort_order") {
		t.Fatal("token_package schema must migrate sort_order used by admin package queries")
	}
}
