package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Jstarzz/claude-meter/internal/config"
	"github.com/Jstarzz/claude-meter/internal/identity"
	"github.com/Jstarzz/claude-meter/internal/server"
	"github.com/Jstarzz/claude-meter/internal/store"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		serve(os.Args[2:])
	case "launch":
		launch(os.Args[2:])
	case "identify":
		identifyCmd(os.Args[2:])
	case "version", "--version", "-version":
		fmt.Println(version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `claude-meter

Usage:
  claude-meter serve    [-config /etc/claude-meter/server.json]
  claude-meter launch   [-config /etc/claude-meter/launcher.json] [claude args...]
  claude-meter identify [-config /etc/claude-meter/launcher.json]
  claude-meter version`)
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cfgPath := fs.String("config", "/etc/claude-meter/server.json", "server config path")
	debug := fs.Bool("debug", false, "debug logging")
	_ = fs.Parse(args)
	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	cfg, err := config.LoadServer(*cfgPath)
	fatal(err)
	st, err := store.Open(cfg.Database)
	fatal(err)
	defer st.Close()
	srv, err := server.New(cfg, st, logger)
	fatal(err)
	stop := make(chan struct{})
	defer close(stop)
	go srv.RunPruner(stop)
	httpSrv := &http.Server{Addr: cfg.Listen, Handler: srv.Handler(), ReadHeaderTimeout: 5_000_000_000}
	go func() {
		logger.Info("claude-meter listening", "address", cfg.Listen, "database", cfg.Database)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logger.Info("shutting down")
	_ = httpSrv.Close()
}

func launch(args []string) {
	fs := flag.NewFlagSet("launch", flag.ExitOnError)
	cfgPath := fs.String("config", "/etc/claude-meter/launcher.json", "launcher config path")
	_ = fs.Parse(args)
	cfg, err := config.LoadLauncher(*cfgPath)
	fatal(err)
	id, err := identity.Resolver{Config: cfg}.Resolve()
	fatal(err)

	env := setEnv(os.Environ(), map[string]string{
		"CLAUDE_CODE_ENABLE_TELEMETRY":     "1",
		"OTEL_LOGS_EXPORTER":               "otlp",
		"OTEL_METRICS_EXPORTER":            "none",
		"OTEL_EXPORTER_OTLP_LOGS_PROTOCOL": "http/json",
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": cfg.Endpoint + "/v1/logs",
		"OTEL_EXPORTER_OTLP_LOGS_HEADERS":  "Authorization=Bearer " + cfg.IngestToken,
		"OTEL_LOG_USER_PROMPTS":            "0",
		"OTEL_LOG_ASSISTANT_RESPONSES":     "0",
		"OTEL_LOG_TOOL_DETAILS":            "0",
		"OTEL_LOG_RAW_API_BODIES":          "0",
		"OTEL_RESOURCE_ATTRIBUTES":         identity.ResourceAttributes(id, os.Getenv("OTEL_RESOURCE_ATTRIBUTES")),
	})
	bin, err := exec.LookPath(cfg.ClaudeBinary)
	fatal(err)
	argv := append([]string{bin}, fs.Args()...)
	if err := syscall.Exec(bin, argv, env); err != nil {
		fatal(err)
	}
}

func identifyCmd(args []string) {
	fs := flag.NewFlagSet("identify", flag.ExitOnError)
	cfgPath := fs.String("config", "/etc/claude-meter/launcher.json", "launcher config path")
	_ = fs.Parse(args)
	cfg, err := config.LoadLauncher(*cfgPath)
	fatal(err)
	id, err := identity.Resolver{Config: cfg}.Resolve()
	fatal(err)
	fmt.Printf("person=%s\ndevice=%s\nnode_id=%s\nsource_ip=%s\n", id.Person, id.Device, id.NodeID, id.SourceIP)
}

func setEnv(base []string, updates map[string]string) []string {
	out := make([]string, 0, len(base)+len(updates))
	seen := make(map[string]bool, len(updates))
	for _, entry := range base {
		key := entry
		if i := strings.IndexByte(entry, '='); i >= 0 {
			key = entry[:i]
		}
		if v, ok := updates[key]; ok {
			if !seen[key] {
				out = append(out, key+"="+v)
				seen[key] = true
			}
			continue
		}
		out = append(out, entry)
	}
	for k, v := range updates {
		if !seen[k] {
			out = append(out, k+"="+v)
		}
	}
	return out
}
func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-meter:", err)
		os.Exit(1)
	}
}
