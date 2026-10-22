package handler

import (
	"net/http"
	"strconv"
	"strings"
)

// ------------------------------------------------------------------ ADMIN PCKG

// AdminPackagesList: GET /api/admin/packages — daftar semua paket token.
func AdminPackagesList(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(),
			`SELECT id, name, token_count, price, duration_days, kind,
			        list_price, promo_price, effective_days, is_active, sort_order
			 FROM token_package ORDER BY kind, sort_order, price`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		type pkg struct {
			ID            string   `json:"id"`
			Name          string   `json:"name"`
			TokenCount    int      `json:"token_count"`
			Price         float64  `json:"price"`
			DurationDays  int      `json:"duration_days"`
			Kind          string   `json:"kind"`
			ListPrice     *float64 `json:"list_price,omitempty"`
			PromoPrice    *float64 `json:"promo_price,omitempty"`
			EffectiveDays *int     `json:"effective_days,omitempty"`
			IsActive      bool     `json:"is_active"`
			SortOrder     int      `json:"sort_order"`
		}
		out := []pkg{}
		for rows.Next() {
			var p pkg
			if err := rows.Scan(&p.ID, &p.Name, &p.TokenCount, &p.Price,
				&p.DurationDays, &p.Kind, &p.ListPrice, &p.PromoPrice,
				&p.EffectiveDays, &p.IsActive, &p.SortOrder); err != nil {
				continue
			}
			out = append(out, p)
		}
		ok(w, out)
	}
}

// AdminPackageGet: GET /api/admin/packages/{id}
func AdminPackageGet(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var p struct {
			ID            string   `json:"id"`
			Name          string   `json:"name"`
			TokenCount    int      `json:"token_count"`
			Price         float64  `json:"price"`
			DurationDays  int      `json:"duration_days"`
			Kind          string   `json:"kind"`
			ListPrice     *float64 `json:"list_price,omitempty"`
			PromoPrice    *float64 `json:"promo_price,omitempty"`
			EffectiveDays *int     `json:"effective_days,omitempty"`
			IsActive      bool     `json:"is_active"`
			SortOrder     int      `json:"sort_order"`
		}
		if err := db.QueryRow(r.Context(),
			`SELECT id, name, token_count, price, duration_days, kind,
			        list_price, promo_price, effective_days, is_active, sort_order
			 FROM token_package WHERE id=$1`, id).Scan(
			&p.ID, &p.Name, &p.TokenCount, &p.Price, &p.DurationDays,
			&p.Kind, &p.ListPrice, &p.PromoPrice, &p.EffectiveDays,
			&p.IsActive, &p.SortOrder); err != nil {
			fail(w, http.StatusNotFound, "paket tidak ditemukan")
			return
		}
		ok(w, p)
	}
}

type input struct {
	Name          string   `json:"name"`
	TokenCount    int      `json:"token_count"`
	Price         float64  `json:"price"`
	DurationDays  int      `json:"duration_days"`
	Kind          string   `json:"kind"`
	ListPrice     *float64 `json:"list_price"`
	PromoPrice    *float64 `json:"promo_price"`
	EffectiveDays *int     `json:"effective_days"`
	IsActive      bool     `json:"is_active"`
	SortOrder     int      `json:"sort_order"`
}

// AdminPackageCreate: POST /api/admin/packages
func AdminPackageCreate(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var in input
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		if in.TokenCount < 0 || in.Kind == "" {
			fail(w, http.StatusBadRequest, "token_count dan kind wajib diisi")
			return
		}
		var id string
		if err := db.QueryRow(r.Context(),
			`INSERT INTO token_package
			 (name, token_count, price, duration_days, kind, list_price, promo_price,
			  effective_days, is_active, sort_order)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
			in.Name, in.TokenCount, in.Price, in.DurationDays, in.Kind,
			in.ListPrice, in.PromoPrice, in.EffectiveDays, in.IsActive, in.SortOrder).
			Scan(&id); err != nil {
			fail(w, http.StatusInternalServerError, "gagal membuat paket")
			return
		}
		audit(r.Context(), db, uid, "PACKAGE_CREATE", id, nil, in, ipOf(r))
		ok(w, map[string]any{"ok": true, "id": id})
	}
}

// AdminPackageUpdate: PATCH /api/admin/packages/{id}
func AdminPackageUpdate(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")
		var in map[string]any
		if err := decode(r, &in); err != nil {
			fail(w, http.StatusBadRequest, "bad json")
			return
		}
		setParts := []string{}
		args := []any{}
		addStr := func(col, key string, idx int, val any) {
			setParts = append(setParts, col+"=$"+strconv.Itoa(idx))
			args = append(args, val)
		}
		idx := 1
		if v, ok := in["name"]; ok {
			addStr("name", "name", idx, v.(string))
			idx++
		}
		if v, ok := in["price"]; ok {
			addStr("price", "price", idx, numToFloat(v))
			idx++
		}
		if v, ok := in["duration_days"]; ok {
			addStr("duration_days", "duration_days", idx, numToInt(v))
			idx++
		}
		if v, ok := in["kind"]; ok {
			addStr("kind", "kind", idx, v.(string))
			idx++
		}
		if v, ok := in["is_active"]; ok {
			addStr("is_active", "is_active", idx, v.(bool))
			idx++
		}
		if v, ok := in["sort_order"]; ok {
			addStr("sort_order", "sort_order", idx, numToInt(v))
			idx++
		}
		if v, ok := in["list_price"]; ok {
			addStr("list_price", "list_price", idx, toNFloat(v))
			idx++
		}
		if v, ok := in["promo_price"]; ok {
			addStr("promo_price", "promo_price", idx, toNFloat(v))
			idx++
		}
		if v, ok := in["effective_days"]; ok {
			addStr("effective_days", "effective_days", idx, toNInt(v))
			idx++
		}
		if len(setParts) == 0 {
			fail(w, http.StatusBadRequest, "tidak ada field yang diupdate")
			return
		}
		args = append(args, id)
		q := `UPDATE token_package SET ` + strings.Join(setParts, ", ") +
			` WHERE id=$` + strconv.Itoa(idx)
		if _, err := db.Exec(r.Context(), q, args...); err != nil {
			fail(w, http.StatusInternalServerError, "gagal update paket")
			return
		}
		audit(r.Context(), db, uid, "PACKAGE_UPDATE", id, nil, in, ipOf(r))
		ok(w, map[string]any{"ok": true, "id": id})
	}
}

// AdminPackageDelete: DELETE /api/admin/packages/{id}
func AdminPackageDelete(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		id := r.PathValue("id")
		// Jangan hapus paket yang masih direferensikan user_token_package.
		var cnt int
		_ = db.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM user_token_package WHERE token_package_id=$1`, id).Scan(&cnt)
		if cnt > 0 {
			fail(w, http.StatusConflict, "paket masih dipakai, tidak bisa dihapus")
			return
		}
		if _, err := db.Exec(r.Context(), `DELETE FROM token_package WHERE id=$1`, id); err != nil {
			fail(w, http.StatusInternalServerError, "gagal hapus paket")
			return
		}
		audit(r.Context(), db, uid, "PACKAGE_DELETE", id, nil, nil, ipOf(r))
		ok(w, map[string]any{"ok": true, "id": id})
	}
}

func numToFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	}
	return 0
}

func numToInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case string:
		i, _ := strconv.Atoi(t)
		return i
	}
	return 0
}

func toNFloat(v any) any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case float64:
		return t
	case string:
		if t == "" {
			return nil
		}
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return nil
		}
		return f
	}
	return nil
}

func toNInt(v any) any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case float64:
		if t == 0 {
			return nil
		}
		return int(t)
	case string:
		if t == "" {
			return nil
		}
		i, _ := strconv.Atoi(t)
		return i
	}
	return nil
}
