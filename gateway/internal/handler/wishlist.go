package handler

import (
	"net/http"
	"strings"
	"time"
)

type wishlistItem struct {
	ID            string    `json:"id"`
	StoreID       string    `json:"store_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Price         float64   `json:"price"`
	Stock         int       `json:"stock"`
	IsDemo        bool      `json:"is_demo"`
	IsInstant     bool      `json:"is_instant"`
	SalesCount    int       `json:"sales_count"`
	ReviewCount   int       `json:"review_count"`
	ImageURL      string    `json:"image_url,omitempty"`
	StoreName     string    `json:"store_name,omitempty"`
	StoreVerified bool      `json:"store_verified"`
	CreatedAt     time.Time `json:"created_at"`
	SavedAt       time.Time `json:"saved_at"`
}

// ListWishlist returns only currently visible products. A deleted or inactive
// listing should disappear from the buyer's list rather than becoming a dead
// card that can no longer be opened.
func ListWishlist(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		rows, err := db.Query(r.Context(), `
			SELECT p.id, COALESCE(p.store_id, p.seller_id), p.title,
			       COALESCE(p.description,''), p.price, p.stock, p.is_demo, p.is_instant,
			       p.sales_count, COALESCE((SELECT COUNT(*) FROM market_review mr WHERE mr.product_id=p.id),0),
			       COALESCE(p.image_url,''), COALESCE(s.name,''),
			       COALESCE((s.is_verified AND (s.verified_until IS NULL OR s.verified_until > now())), false),
			       p.created_at, w.created_at
			  FROM market_wishlist w
			  JOIN market_product p ON p.id=w.product_id
			  LEFT JOIN market_store s ON s.id=p.store_id
			 WHERE w.user_id=$1 AND p.is_active=true AND p.deleted_at IS NULL
			   AND p.is_demo=false AND COALESCE(s.is_active,true)=true
			 ORDER BY w.created_at DESC`, uid)
		if err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		items := []wishlistItem{}
		for rows.Next() {
			var item wishlistItem
			if err := rows.Scan(&item.ID, &item.StoreID, &item.Title, &item.Description,
				&item.Price, &item.Stock, &item.IsDemo, &item.IsInstant, &item.SalesCount,
				&item.ReviewCount, &item.ImageURL, &item.StoreName, &item.StoreVerified,
				&item.CreatedAt, &item.SavedAt); err != nil {
				continue
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			fail(w, http.StatusInternalServerError, "query failed")
			return
		}
		ok(w, map[string]any{"items": items})
	}
}

// AddWishlist is idempotent. The visibility check is server-side so a client
// cannot save demo, deleted, inactive, or closed-store listings.
func AddWishlist(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		productID := strings.TrimSpace(r.PathValue("id"))
		if productID == "" {
			fail(w, http.StatusBadRequest, "product_id wajib")
			return
		}
		var visibleID string
		err := db.QueryRow(r.Context(), `
			SELECT p.id::text
			  FROM market_product p
			  LEFT JOIN market_store s ON s.id=p.store_id
			 WHERE p.id=$1 AND p.is_active=true AND p.deleted_at IS NULL
			   AND p.is_demo=false AND COALESCE(s.is_active,true)=true`, productID).Scan(&visibleID)
		if err != nil {
			fail(w, http.StatusNotFound, "produk tidak ditemukan")
			return
		}
		if _, err := db.Exec(r.Context(), `
			INSERT INTO market_wishlist (user_id, product_id)
			VALUES ($1,$2) ON CONFLICT (user_id, product_id) DO NOTHING`, uid, visibleID); err != nil {
			fail(w, http.StatusInternalServerError, "gagal menyimpan favorit")
			return
		}
		ok(w, map[string]any{"ok": true, "saved": true, "product_id": visibleID})
	}
}

func RemoveWishlist(db DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromCtx(r.Context())
		productID := strings.TrimSpace(r.PathValue("id"))
		if productID == "" {
			fail(w, http.StatusBadRequest, "product_id wajib")
			return
		}
		result, err := db.Exec(r.Context(), `DELETE FROM market_wishlist WHERE user_id=$1 AND product_id=$2`, uid, productID)
		if err != nil {
			fail(w, http.StatusInternalServerError, "gagal menghapus favorit")
			return
		}
		ok(w, map[string]any{"ok": true, "saved": false, "removed": result.RowsAffected() > 0, "product_id": productID})
	}
}
