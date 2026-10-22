package handler

import "testing"

func TestValidateRegistration(t *testing.T) {
	valid := registerInput{Email: " User@Example.COM ", FirstName: " Budi ", LastName: " Santoso ", Password: "password123", PasswordConfirm: "password123", AffCode: " ref01 "}
	tests := []struct {
		name string
		in   registerInput
		want string
	}{
		{"valid", valid, ""},
		{"email", registerInput{Email: "bukan-email", FirstName: "Budi", LastName: "Santoso", Password: "password123", PasswordConfirm: "password123"}, "format email tidak valid"},
		{"first", registerInput{Email: "a@b.com", FirstName: "B", LastName: "Santoso", Password: "password123", PasswordConfirm: "password123"}, "nama lengkap wajib 2-120 karakter"},
		{"single name", registerInput{Email: "a@b.com", FirstName: "Budi", Password: "password123", PasswordConfirm: "password123"}, ""},
		{"last", registerInput{Email: "a@b.com", FirstName: "Budi", LastName: string(make([]byte, 121)), Password: "password123", PasswordConfirm: "password123"}, "nama lengkap wajib 2-120 karakter"},
		{"short password", registerInput{Email: "a@b.com", FirstName: "Budi", LastName: "Santoso", Password: "pendek", PasswordConfirm: "pendek"}, "password wajib 8-72 karakter"},
		{"long password", registerInput{Email: "a@b.com", FirstName: "Budi", LastName: "Santoso", Password: string(make([]byte, 73)), PasswordConfirm: string(make([]byte, 73))}, "password wajib 8-72 karakter"},
		{"mismatch", registerInput{Email: "a@b.com", FirstName: "Budi", LastName: "Santoso", Password: "password123", PasswordConfirm: "password124"}, "verifikasi password tidak sama"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, errMsg := validateRegistration(tc.in)
			if errMsg != tc.want {
				t.Fatalf("error=%q, want %q", errMsg, tc.want)
			}
			if tc.name == "valid" && (got.Email != "user@example.com" || got.FirstName != "Budi" || got.LastName != "Santoso" || got.AffCode != "REF01") {
				t.Fatalf("normalisasi salah: %+v", got)
			}
		})
	}
}

func TestValidationCode(t *testing.T) {
	cases := map[string]string{
		"format email tidak valid":          "email_invalid",
		"nama lengkap wajib 2-120 karakter": "name_invalid",
		"password wajib 8-72 karakter":      "password_invalid",
		"verifikasi password tidak sama":    "password_mismatch",
		"lain":                              "validation_failed",
	}
	for message, want := range cases {
		if got := validationCode(message); got != want {
			t.Fatalf("validationCode(%q)=%q, want %q", message, got, want)
		}
	}
}
