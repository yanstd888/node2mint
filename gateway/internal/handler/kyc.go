package handler

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"ai-sell/gateway/internal/secret"
)

type kycAdminItem struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	DocType   string    `json:"doc_type"`
	Name      string    `json:"name"`
	Mime      string    `json:"mime"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
	Email     string    `json:"email"`
}

func kycConsentAccepted(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func KYCStatus(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		var status, reason string
		_ = db.QueryRow(r.Context(), `SELECT kyc_status,COALESCE(kyc_rejection_reason,'') FROM "user" WHERE id=$1`, uid).Scan(&status, &reason)
		ok(w, map[string]any{"status": status, "rejection_reason": reason})
	}
}

func KYCSubmit(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		r.Body = http.MaxBytesReader(w, r.Body, 5<<20+1024)
		if err := r.ParseMultipartForm(5 << 20); err != nil {
			fail(w, http.StatusBadRequest, "file maksimal 5 MB")
			return
		}
		docType := strings.ToLower(strings.TrimSpace(r.FormValue("doc_type")))
		if docType != "ktp" && docType != "passport" {
			fail(w, http.StatusBadRequest, "doc_type harus ktp atau passport")
			return
		}
		if !kycConsentAccepted(r.FormValue("consent")) {
			fail(w, http.StatusBadRequest, "persetujuan pemrosesan data wajib")
			return
		}
		file, hdr, err := r.FormFile("document")
		if err != nil || hdr.Size == 0 || hdr.Size > 5<<20 {
			fail(w, http.StatusBadRequest, "dokumen wajib dan maksimal 5 MB")
			return
		}
		defer file.Close()
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(file); err != nil {
			fail(w, http.StatusBadRequest, "dokumen tidak terbaca")
			return
		}
		mime := strings.ToLower(strings.TrimSpace(hdr.Header.Get("Content-Type")))
		if mime == "" {
			mime = http.DetectContentType(buf.Bytes())
		}
		if mime != "application/pdf" && mime != "image/jpeg" && mime != "image/png" {
			fail(w, http.StatusBadRequest, "format harus PDF, JPG, atau PNG")
			return
		}
		ciphertext, err := secret.Encrypt(base64.StdEncoding.EncodeToString(buf.Bytes()))
		if err != nil {
			fail(w, http.StatusInternalServerError, "enkripsi dokumen gagal")
			return
		}
		_, _ = db.Exec(r.Context(), `UPDATE "user" SET kyc_status='pending', kyc_rejection_reason=NULL, kyc_reviewed_at=NULL WHERE id=$1`, uid)
		var id string
		if err := db.QueryRow(r.Context(), `INSERT INTO kyc_verification(user_id,doc_type,original_name,mime_type,encrypted_data) VALUES($1,$2,$3,$4,$5) RETURNING id`, uid, docType, hdr.Filename, mime, ciphertext).Scan(&id); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan verifikasi")
			return
		}
		audit(r.Context(), db, uid, "KYC_SUBMIT", id, nil, map[string]any{"doc_type": docType, "mime_type": mime}, ipOf(r))
		ok(w, map[string]any{"ok": true, "id": id, "status": "pending"})
	}
}

func AdminKYCList(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(), `SELECT k.id,k.user_id,k.doc_type,k.original_name,k.mime_type,k.status,COALESCE(k.rejection_reason,''),k.created_at,u.email FROM kyc_verification k JOIN "user" u ON u.id=k.user_id WHERE k.status='pending' ORDER BY k.created_at ASC LIMIT 100`)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		out := []kycAdminItem{}
		for rows.Next() {
			var x kycAdminItem
			if err := rows.Scan(&x.ID, &x.UserID, &x.DocType, &x.Name, &x.Mime, &x.Status, &x.Reason, &x.CreatedAt, &x.Email); err != nil {
				fail(w, http.StatusInternalServerError, "query failed")
				return
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		ok(w, map[string]any{"requests": out})
	}
}

func AdminKYCDecision(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		action := r.PathValue("action")
		if action != "approve" && action != "reject" {
			fail(w, 400, "action invalid")
			return
		}
		var reason string
		if action == "reject" {
			var in struct {
				Reason string `json:"reason"`
			}
			if decode(r, &in) != nil || len(strings.TrimSpace(in.Reason)) < 3 {
				fail(w, 400, "reason wajib")
				return
			}
			reason = strings.TrimSpace(in.Reason)
		}
		status := "approved"
		kycStatus := "verified"
		if action == "reject" {
			status = "rejected"
			kycStatus = "unverified"
		}
		admin := uidFromCtx(r.Context())
		var userID string
		if err := db.QueryRow(r.Context(), `UPDATE kyc_verification SET status=$1,rejection_reason=$2,reviewed_by=$3,reviewed_at=now() WHERE id=$4 AND status='pending' RETURNING user_id`, status, reason, admin, id).Scan(&userID); err != nil {
			fail(w, 404, "request tidak ditemukan")
			return
		}
		_, _ = db.Exec(r.Context(), `UPDATE "user" SET kyc_status=$1,kyc_rejection_reason=$2,kyc_reviewed_at=now() WHERE id=$3`, kycStatus, reason, userID)
		audit(r.Context(), db, admin, "KYC_"+strings.ToUpper(action), id, userID, map[string]any{"reason": reason}, ipOf(r))
		ok(w, map[string]any{"ok": true, "status": status})
	}
}

func AdminKYCDocument(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var ciphertext, mime, name string
		if err := db.QueryRow(r.Context(), `SELECT encrypted_data,mime_type,original_name FROM kyc_verification WHERE id=$1`, id).Scan(&ciphertext, &mime, &name); err != nil {
			fail(w, http.StatusNotFound, "document tidak ditemukan")
			return
		}
		encoded, err := secret.Decrypt(ciphertext)
		if err != nil {
			fail(w, http.StatusInternalServerError, "document tidak dapat dibuka")
			return
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			fail(w, http.StatusInternalServerError, "document rusak")
			return
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Content-Disposition", `inline; filename="`+strings.ReplaceAll(name, `"`, "")+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}
