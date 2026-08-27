package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jstarzz/claude-meter/internal/config"
	"github.com/Jstarzz/claude-meter/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "usage.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := New(config.Server{Title: "Claude Meter"}, st, logger)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestDashboardTemplateParses(t *testing.T) {
	_ = newTestServer(t)
}

func TestDashboardControlsRender(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/?range=7d", nil)
	res := httptest.NewRecorder()
	srv.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"data-theme-toggle", "data-refresh", "#overview", "#efficiency", "data-efficiency", "/static/app.js"} {
		if !strings.Contains(body, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}

func TestDashboardScriptAndCSP(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	res := httptest.NewRecorder()
	srv.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "claude-meter-theme") {
		t.Fatal("theme behavior missing")
	}
	if !strings.Contains(body, "cacheReuseRate") {
		t.Fatal("efficiency calculation missing")
	}
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/javascript") {
		t.Fatalf("content type %q", got)
	}
	csp := res.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf("CSP does not allow same-origin script: %q", csp)
	}
	if strings.Contains(csp, "script-src 'none'") {
		t.Fatalf("CSP still blocks scripts: %q", csp)
	}
}
