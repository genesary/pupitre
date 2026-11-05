package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandler_ReturnsMetrics verifies the metrics handler serves output
// containing the expected metric names.
func TestHandler_ReturnsMetrics(t *testing.T) {
	t.Parallel()

	h := Handler()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	for _, name := range []string{"pupitre_active_users_total", "pupitre_active_courses_total", "pupitre_enrollments_total"} {
		if !strings.Contains(body, name) {
			t.Errorf("expected %s in metrics output", name)
		}
	}
}
