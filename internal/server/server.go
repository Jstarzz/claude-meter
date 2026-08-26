package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Jstarzz/claude-meter/internal/config"
	"github.com/Jstarzz/claude-meter/internal/otel"
	"github.com/Jstarzz/claude-meter/internal/store"
	webassets "github.com/Jstarzz/claude-meter/internal/web"
)

type Server struct {
	cfg   config.Server
	store *store.Store
	log   *slog.Logger
	tpl   *template.Template
}

type dashboardData struct {
	Title     string
	Range     string
	Person    string
	Totals    store.Totals
	People    []store.PersonUsage
	Accounts  []store.AccountUsage
	History   []store.HistoryRow
	Generated time.Time
}

func New(cfg config.Server, st *store.Store, logger *slog.Logger) (*Server, error) {
	funcs := template.FuncMap{
		"money":  func(m int64) string { return fmt.Sprintf("$%.2f", float64(m)/1_000_000) },
		"tokens": formatTokens,
		"short":  shortID,
		"add":    func(a, b int) int { return a + b },
		"ago":    func(t time.Time) string { return humanAgo(time.Since(t)) },
		"ms": func(v int64) string {
			if v >= 1000 {
				return fmt.Sprintf("%.1fs", float64(v)/1000)
			}
			return fmt.Sprintf("%dms", v)
		},
	}
	tpl, err := template.New("index.html").Funcs(funcs).ParseFS(webassets.Files, "templates/index.html")
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, store: st, log: logger, tpl: tpl}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/logs", s.ingestLogs)
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/summary", s.summaryAPI)
	mux.HandleFunc("GET /api/history", s.historyAPI)
	mux.HandleFunc("GET /static/style.css", s.style)
	mux.HandleFunc("GET /", s.dashboard)
	return securityHeaders(mux)
}

func (s *Server) ingestLogs(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	events, err := otel.Decode(body, time.Now())
	if err != nil {
		s.log.Warn("rejecting OTLP payload", "error", err)
		http.Error(w, "invalid OTLP JSON", http.StatusBadRequest)
		return
	}
	inserted, err := s.store.Insert(r.Context(), events)
	if err != nil {
		s.log.Error("persist telemetry", "error", err)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	s.log.Debug("telemetry batch", "events", len(events), "inserted", inserted)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	rng := normalizeRange(r.URL.Query().Get("range"))
	person := strings.TrimSpace(r.URL.Query().Get("person"))
	since := rangeStart(rng, time.Now())
	totals, err := s.store.Totals(r.Context(), since, person)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	people, err := s.store.ByPerson(r.Context(), since)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	accounts, err := s.store.ByAccount(r.Context(), since, person)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	history, err := s.store.History(r.Context(), since, person, 100)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	data := dashboardData{Title: s.cfg.Title, Range: rng, Person: person, Totals: totals, People: people, Accounts: accounts, History: history, Generated: time.Now()}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tpl.ExecuteTemplate(w, "index.html", data); err != nil {
		s.log.Error("render dashboard", "error", err)
	}
}

func (s *Server) summaryAPI(w http.ResponseWriter, r *http.Request) {
	rng := normalizeRange(r.URL.Query().Get("range"))
	person := strings.TrimSpace(r.URL.Query().Get("person"))
	since := rangeStart(rng, time.Now())
	totals, err := s.store.Totals(r.Context(), since, person)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	people, _ := s.store.ByPerson(r.Context(), since)
	accounts, _ := s.store.ByAccount(r.Context(), since, person)
	writeJSON(w, map[string]any{"range": rng, "person": person, "totals": totals, "people": people, "accounts": accounts})
}

func (s *Server) historyAPI(w http.ResponseWriter, r *http.Request) {
	rng := normalizeRange(r.URL.Query().Get("range"))
	person := strings.TrimSpace(r.URL.Query().Get("person"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.store.History(r.Context(), rangeStart(rng, time.Now()), person, limit)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	writeJSON(w, rows)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		http.Error(w, "database unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *Server) style(w http.ResponseWriter, r *http.Request) {
	b, err := webassets.Files.ReadFile("static/style.css")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}

func (s *Server) authorized(r *http.Request) bool {
	got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	expected := s.cfg.IngestToken
	if len(got) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

func (s *Server) RunPruner(stop <-chan struct{}) {
	if s.cfg.RetentionDays <= 0 {
		return
	}
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			n, err := s.store.Prune(context.Background(), s.cfg.RetentionDays)
			if err != nil {
				s.log.Warn("prune history", "error", err)
				continue
			}
			if n > 0 {
				s.log.Info("pruned old history", "rows", n)
			}
		}
	}
}

func normalizeRange(v string) string {
	switch v {
	case "24h", "7d", "30d", "all":
		return v
	default:
		return "7d"
	}
}
func rangeStart(rng string, now time.Time) *time.Time {
	var t time.Time
	switch rng {
	case "24h":
		t = now.Add(-24 * time.Hour)
	case "7d":
		t = now.Add(-7 * 24 * time.Hour)
	case "30d":
		t = now.Add(-30 * 24 * time.Hour)
	default:
		return nil
	}
	t = t.UTC()
	return &t
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func formatTokens(v int64) string {
	switch {
	case v >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", float64(v)/1e9)
	case v >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(v)/1e6)
	case v >= 1_000:
		return fmt.Sprintf("%.1fK", float64(v)/1e3)
	default:
		return strconv.FormatInt(v, 10)
	}
}
func shortID(v string) string {
	if len(v) > 12 {
		return v[:12] + "..."
	}
	if v == "" {
		return "-"
	}
	return v
}
func humanAgo(d time.Duration) string {
	if d < time.Minute {
		return "now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'none'; img-src 'self' data:; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
