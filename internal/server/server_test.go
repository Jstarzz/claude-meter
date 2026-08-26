package server

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/Jstarzz/claude-meter/internal/config"
	"github.com/Jstarzz/claude-meter/internal/store"
)

func TestDashboardTemplateParses(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "usage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := New(config.Server{Title: "Claude Meter"}, st, logger); err != nil {
		t.Fatal(err)
	}
}
