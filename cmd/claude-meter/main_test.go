package main

import "testing"

func TestParseLaunchArgsPassesClaudeFlags(t *testing.T) {
	cfg, fallback, args := parseLaunchArgs([]string{
		"-config", "/etc/test.json",
		"-fallback-bin", "/real/claude",
		"--",
		"--version",
	})
	if cfg != "/etc/test.json" || fallback != "/real/claude" {
		t.Fatalf("got cfg=%q fallback=%q", cfg, fallback)
	}
	if len(args) != 1 || args[0] != "--version" {
		t.Fatalf("got args=%v", args)
	}
}

func TestParseLaunchArgsStopsAtClaudeArgs(t *testing.T) {
	cfg, fallback, args := parseLaunchArgs([]string{"-config=/etc/test.json", "--dangerously-skip-permissions", "prompt"})
	if cfg != "/etc/test.json" || fallback != "" {
		t.Fatalf("got cfg=%q fallback=%q", cfg, fallback)
	}
	if len(args) != 2 || args[0] != "--dangerously-skip-permissions" || args[1] != "prompt" {
		t.Fatalf("got args=%v", args)
	}
}

func TestParseLaunchArgsDefaults(t *testing.T) {
	cfg, fallback, args := parseLaunchArgs([]string{"prompt"})
	if cfg != "/etc/claude-meter/launcher.json" || fallback != "" {
		t.Fatalf("got cfg=%q fallback=%q", cfg, fallback)
	}
	if len(args) != 1 || args[0] != "prompt" {
		t.Fatalf("got args=%v", args)
	}
}
