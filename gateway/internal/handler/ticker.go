package handler

// Ticker logo di landing: pita logo berjalan yang dikelola admin.
// Gambar diupload lewat /api/media (sudah ada), image_url menunjuk /api/media/{id}.

import (
	"net/http"
	"strings"
)

type tickerLogo struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	ImageURL string `json:"image_url"`
	Sort     int    `json:"sort"`
	IsActive bool   `json:"is_active"`
}

// TickerList: GET /api/ticker — publik, hanya yang aktif.
func TickerList(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(),
			`SELECT id, COALESCE(label,''), COALESCE(image_url,''), sort, is_active
			 FROM ticker_logo WHERE is_active ORDER BY sort, created_at`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		out := []tickerLogo{}
		for rows.Next() {
			var t tickerLogo
			if err := rows.Scan(&t.ID, &t.Label, &t.ImageURL, &t.Sort, &t.IsActive); err != nil {
				fail(w, http.StatusInternalServerError, "scan failed")
				return
			}
			out = append(out, t)
		}
		ok(w, map[string]any{"logos": out})
	}
}

// AdminTickerList: GET /api/admin/ticker — semua, termasuk nonaktif.
func AdminTickerList(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(),
			`SELECT id, COALESCE(label,''), COALESCE(image_url,''), sort, is_active
			 FROM ticker_logo ORDER BY sort, created_at`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		out := []tickerLogo{}
		for rows.Next() {
			var t tickerLogo
			if err := rows.Scan(&t.ID, &t.Label, &t.ImageURL, &t.Sort, &t.IsActive); err != nil {
				fail(w, http.StatusInternalServerError, "scan failed")
				return
			}
			out = append(out, t)
		}
		ok(w, map[string]any{"logos": out})
	}
}

// AdminTickerCreate: POST /api/admin/ticker {label, image_url, sort}.
func AdminTickerCreate(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Label    string `json:"label"`
			ImageURL string `json:"image_url"`
			Sort     int    `json:"sort"`
		}
		if err := decode(r, &in); err != nil || strings.TrimSpace(in.ImageURL) == "" {
			fail(w, http.StatusBadRequest, "image_url wajib")
			return
		}
		var id string
		err := db.QueryRow(r.Context(),
			`INSERT INTO ticker_logo (label, image_url, sort) VALUES ($1,$2,$3) RETURNING id`,
			strings.TrimSpace(in.Label), strings.TrimSpace(in.ImageURL), in.Sort).Scan(&id)
		if err != nil {
			fail(w, http.StatusInternalServerError, "insert failed")
			return
		}
		uid := uidFromCtx(r.Context())
		ok(w, map[string]any{"ok": true, "id": id})
		audit(r.Context(), db, uid, "TICKER_CREATE", id, nil, map[string]any{"label": in.Label}, ipOf(r))
	}
}

// AdminTickerUpdate: PATCH /api/admin/ticker {id, label?, image_url?, sort?, is_active?}.
func AdminTickerUpdate(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID       string  `json:"id"`
			Label    *string `json:"label"`
			ImageURL *string `json:"image_url"`
			Sort     *int    `json:"sort"`
			IsActive *bool   `json:"is_active"`
		}
		if err := decode(r, &in); err != nil || in.ID == "" {
			fail(w, http.StatusBadRequest, "id wajib")
			return
		}
		sets, args := []string{}, []any{in.ID}
		add := func(sql string, v any) { sets = append(sets, sql); args = append(args, v) }
		if in.Label != nil {
			add("label=$"+itoa(len(args)+1), strings.TrimSpace(*in.Label))
		}
		if in.ImageURL != nil {
			add("image_url=$"+itoa(len(args)+1), strings.TrimSpace(*in.ImageURL))
		}
		if in.Sort != nil {
			add("sort=$"+itoa(len(args)+1), *in.Sort)
		}
		if in.IsActive != nil {
			add("is_active=$"+itoa(len(args)+1), *in.IsActive)
		}
		if len(sets) == 0 {
			fail(w, http.StatusBadRequest, "tidak ada perubahan")
			return
		}
		ct, err := db.Exec(r.Context(),
			`UPDATE ticker_logo SET `+strings.Join(sets, ", ")+` WHERE id=$1`, args...)
		if err != nil {
			fail(w, http.StatusInternalServerError, "update failed")
			return
		}
		if ct.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "logo tidak ditemukan")
			return
		}
		uid := uidFromCtx(r.Context())
		ok(w, map[string]any{"ok": true})
		audit(r.Context(), db, uid, "TICKER_UPDATE", in.ID, nil, map[string]any{"active": in.IsActive != nil}, ipOf(r))
	}
}

// AdminTickerDelete: DELETE /api/admin/ticker/{id}.
func AdminTickerDelete(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ct, err := db.Exec(r.Context(), `DELETE FROM ticker_logo WHERE id=$1`, r.PathValue("id"))
		if err != nil {
			fail(w, http.StatusInternalServerError, "delete failed")
			return
		}
		if ct.RowsAffected() == 0 {
			fail(w, http.StatusNotFound, "logo tidak ditemukan")
			return
		}
		uid := uidFromCtx(r.Context())
		ok(w, map[string]any{"ok": true})
		audit(r.Context(), db, uid, "TICKER_DELETE", r.PathValue("id"), nil, nil, ipOf(r))
	}
}
