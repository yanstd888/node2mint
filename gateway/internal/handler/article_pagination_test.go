package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"

	"ai-sell/gateway/internal/store"
)

// TestListArticlesReportsFullFilteredTotal guards the public pagination
// contract with an explicit disposable DB gate, never a production fixture.
func TestListArticlesReportsFullFilteredTotal(t *testing.T) {
	if os.Getenv("ARTICLE_PAGINATION_TEST") != "1" || os.Getenv("DATABASE_URL") == "" {
		t.Skip("ARTICLE_PAGINATION_TEST=1 dan DATABASE_URL disposable diperlukan")
	}
	ctx := context.Background()
	db, err := store.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	prefix := "article-pagination-" + randCode(8)
	t.Cleanup(func() {
		if _, err := db.Exec(ctx, `DELETE FROM article WHERE slug LIKE $1`, prefix+"-%"); err != nil {
			t.Errorf("bersihkan artikel fixture: %v", err)
		}
	})
	for i := 0; i < 21; i++ {
		slug := prefix + "-" + strconv.Itoa(i)
		if _, err := db.Exec(ctx,
			`INSERT INTO article (title, slug, content, status, published_at)
			 VALUES ($1,$2,'fixture','PUBLISHED',now())`, prefix, slug); err != nil {
			t.Fatalf("seed artikel %d: %v", i, err)
		}
	}

	type response struct {
		Articles []json.RawMessage `json:"articles"`
		Total    int               `json:"total"`
		Page     int               `json:"page"`
		HasMore  bool              `json:"has_more"`
	}
	check := func(page, wantRows int, wantMore bool) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet,
			"/api/articles?q="+url.QueryEscape(prefix)+"&per_page=20&page="+strconv.Itoa(page), nil)
		rec := httptest.NewRecorder()
		ListArticles(db).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("page %d status=%d, mau 200 (body=%s)", page, rec.Code, rec.Body.String())
		}
		var got response
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode page %d: %v (body=%s)", page, err, rec.Body.String())
		}
		if got.Total != 21 || got.Page != page || len(got.Articles) != wantRows || got.HasMore != wantMore {
			t.Fatalf("page %d: total=%d page=%d rows=%d has_more=%t; mau 21/%d/%d/%t",
				page, got.Total, got.Page, len(got.Articles), got.HasMore, page, wantRows, wantMore)
		}
	}
	check(1, 20, true)
	check(2, 1, false)
}
