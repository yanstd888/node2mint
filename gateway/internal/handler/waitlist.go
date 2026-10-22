package handler

import (
	"net/http"
	"regexp"
	"strings"
)

var waitlistEmail = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func JoinWaitlist(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Email string `json:"email"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		email := strings.ToLower(strings.TrimSpace(in.Email))
		if len(email) > 254 || !waitlistEmail.MatchString(email) {
			fail(w, http.StatusBadRequest, "invalid email")
			return
		}
		if _, err := db.Exec(r.Context(), `INSERT INTO waitlist (email) VALUES ($1) ON CONFLICT (email) DO NOTHING`, email); err != nil {
			fail(w, http.StatusInternalServerError, "save failed")
			return
		}
		ok(w, map[string]any{"ok": true})
	}
}
