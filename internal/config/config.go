package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Server struct {
	Listen        string `json:"listen"`
	Database      string `json:"database"`
	IngestToken   string `json:"ingest_token"`
	RetentionDays int    `json:"retention_days"`
	Title         string `json:"title"`
}

type Launcher struct {
	Endpoint     string            `json:"endpoint"`
	IngestToken  string            `json:"ingest_token"`
	ClaudeBinary string            `json:"claude_binary"`
	TailscaleBin string            `json:"tailscale_binary"`
	LocalPerson  string            `json:"local_person"`
	Devices      map[string]string `json:"devices"`
}

func LoadServer(path string) (Server, error) {
	cfg := Server{
		Listen:   "0.0.0.0:8787",
		Database: "/var/lib/claude-meter/usage.db",
		Title:    "Claude Meter",
	}
	if err := loadJSON(path, &cfg); err != nil {
		return Server{}, err
	}
	if v := os.Getenv("CLAUDE_METER_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("CLAUDE_METER_DATABASE"); v != "" {
		cfg.Database = v
	}
	if v := os.Getenv("CLAUDE_METER_INGEST_TOKEN"); v != "" {
		cfg.IngestToken = v
	}
	if strings.TrimSpace(cfg.IngestToken) == "" {
		return Server{}, errors.New("ingest_token is required")
	}
	return cfg, nil
}

func LoadLauncher(path string) (Launcher, error) {
	cfg := Launcher{TailscaleBin: "tailscale"}
	if err := loadJSON(path, &cfg); err != nil {
		return Launcher{}, err
	}
	if v := os.Getenv("CLAUDE_METER_ENDPOINT"); v != "" {
		cfg.Endpoint = v
	}
	if v := os.Getenv("CLAUDE_METER_INGEST_TOKEN"); v != "" {
		cfg.IngestToken = v
	}
	if cfg.TailscaleBin == "" {
		cfg.TailscaleBin = "tailscale"
	}
	cfg.Endpoint = strings.TrimRight(cfg.Endpoint, "/")
	if cfg.Endpoint == "" || cfg.IngestToken == "" || cfg.ClaudeBinary == "" {
		return Launcher{}, errors.New("endpoint, ingest_token, and claude_binary are required")
	}
	if len(cfg.Devices) == 0 && cfg.LocalPerson == "" {
		return Launcher{}, errors.New("at least one device mapping or local_person is required")
	}
	normalized := make(map[string]string, len(cfg.Devices))
	for device, person := range cfg.Devices {
		d := normalizeDevice(device)
		p := strings.TrimSpace(person)
		if d == "" || p == "" {
			return Launcher{}, fmt.Errorf("invalid device mapping %q -> %q", device, person)
		}
		normalized[d] = p
	}
	cfg.Devices = normalized
	return cfg, nil
}

func loadJSON(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	return nil
}

func normalizeDevice(s string) string {
	s = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(s, ".")))
	if i := strings.IndexByte(s, '.'); i > 0 {
		s = s[:i]
	}
	return s
}

func NormalizeDevice(s string) string { return normalizeDevice(s) }
