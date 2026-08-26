package identity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Jstarzz/claude-meter/internal/config"
)

func TestParseWhois(t *testing.T) {
	raw := []byte(`{"Node":{"ID":123,"StableID":"nAbCd","ComputedName":"desktop-josiah"},"UserProfile":{"LoginName":"info@example.com"}}`)
	device, nodeID, err := parseWhois(raw)
	if err != nil {
		t.Fatal(err)
	}
	if device != "desktop-josiah" || nodeID != "nAbCd" {
		t.Fatalf("got %q %q", device, nodeID)
	}
}

func TestSourceIPFromSSH(t *testing.T) {
	for _, v := range []string{
		"100.108.157.80 53124 100.109.68.120 22",
		"100.108.157.80 53124 22",
		"100.108.157.80",
	} {
		if got := sourceIPFromSSH(v); got != "100.108.157.80" {
			t.Fatalf("got %q from %q", got, v)
		}
	}
	if got := sourceIPFromSSH("not-an-ip 1 2 3"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveLocalUnknown(t *testing.T) {
	clearRemoteEnv(t)
	id, err := (Resolver{Config: config.Launcher{}}).Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if id.Person != "unknown" || id.Device != "local" || id.SourceIP != "" {
		t.Fatalf("got %+v", id)
	}
}

func TestResolveLocalPerson(t *testing.T) {
	clearRemoteEnv(t)
	id, err := (Resolver{Config: config.Launcher{LocalPerson: "josiah"}}).Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if id.Person != "josiah" || id.Device != "local" {
		t.Fatalf("got %+v", id)
	}
}

func TestResolveWhoisFailure(t *testing.T) {
	clearRemoteEnv(t)
	t.Setenv("SSH_CONNECTION", "192.0.2.10 50000 192.0.2.20 22")
	id, err := (Resolver{Config: config.Launcher{TailscaleBin: filepath.Join(t.TempDir(), "missing")}}).Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if id.Person != "unknown" || id.Device != "unresolved" || id.SourceIP != "192.0.2.10" {
		t.Fatalf("got %+v", id)
	}
}

func TestResolveUnknownDevice(t *testing.T) {
	clearRemoteEnv(t)
	t.Setenv("SSH_CONNECTION", "100.64.0.10 50000 100.64.0.20 22")
	bin := fakeCommand(t, `{"Node":{"StableID":"node-1","ComputedName":"new-laptop"}}`)
	id, err := (Resolver{Config: config.Launcher{TailscaleBin: bin, Devices: map[string]string{}}}).Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if id.Person != "unknown" || id.Device != "new-laptop" || id.NodeID != "node-1" || id.SourceIP != "100.64.0.10" {
		t.Fatalf("got %+v", id)
	}
}

func TestResolveKnownDevice(t *testing.T) {
	clearRemoteEnv(t)
	t.Setenv("SSH_CONNECTION", "100.64.0.10 50000 100.64.0.20 22")
	bin := fakeCommand(t, `{"Node":{"StableID":"node-1","ComputedName":"desktop-josiah"}}`)
	id, err := (Resolver{Config: config.Launcher{TailscaleBin: bin, Devices: map[string]string{"desktop-josiah": "josiah"}}}).Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if id.Person != "josiah" || id.Device != "desktop-josiah" {
		t.Fatalf("got %+v", id)
	}
}

func TestTMUXEnvironmentWins(t *testing.T) {
	clearRemoteEnv(t)
	t.Setenv("TMUX", "/tmp/tmux/default,1,0")
	t.Setenv("SSH_CONNECTION", "100.64.0.1 50000 100.64.0.20 22")
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux")
	body := "#!/bin/sh\nprintf '%s\\n' 'SSH_CONNECTION=100.64.0.2 50001 100.64.0.20 22'\n"
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if got := sourceIPFromEnvironment(); got != "100.64.0.2" {
		t.Fatalf("got %q", got)
	}
}

func clearRemoteEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TMUX", "")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("MOSH_IP", "")
}

func fakeCommand(t *testing.T, payload string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tailscale")
	body := "#!/bin/sh\nprintf '%s\\n' '" + payload + "'\n"
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}
