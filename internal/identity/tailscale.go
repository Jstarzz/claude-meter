package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Jstarzz/claude-meter/internal/config"
)

type Identity struct {
	Person   string
	Device   string
	NodeID   string
	SourceIP string
}

type Resolver struct {
	Config config.Launcher
}

func (r Resolver) Resolve() (Identity, error) {
	sourceIP := sourceIPFromEnvironment()
	if sourceIP == "" {
		person := strings.TrimSpace(r.Config.LocalPerson)
		if person == "" {
			person = "unknown"
		}
		return Identity{Person: person, Device: "local"}, nil
	}

	fallback := Identity{Person: "unknown", Device: "unresolved", SourceIP: sourceIP}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.Config.TailscaleBin, "whois", "--json", sourceIP)
	out, err := cmd.Output()
	if err != nil {
		return fallback, nil
	}

	device, nodeID, err := parseWhois(out)
	if err != nil {
		return fallback, nil
	}
	normalized := config.NormalizeDevice(device)
	if normalized == "" {
		return fallback, nil
	}
	fallback.Device = normalized
	fallback.NodeID = nodeID
	if person := strings.TrimSpace(r.Config.Devices[normalized]); person != "" {
		fallback.Person = person
	}
	return fallback, nil
}

func sourceIPFromEnvironment() string {
	if os.Getenv("TMUX") != "" {
		if ip := sourceIPFromTMUX(); ip != "" {
			return ip
		}
	}
	if ip := sourceIPFromSSH(os.Getenv("SSH_CONNECTION")); ip != "" {
		return ip
	}
	if ip := sourceIPFromSSH(os.Getenv("SSH_CLIENT")); ip != "" {
		return ip
	}
	if ip := net.ParseIP(strings.TrimSpace(os.Getenv("MOSH_IP"))); ip != nil {
		return ip.String()
	}
	return ""
}

func sourceIPFromTMUX() string {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	for _, key := range []string{"SSH_CONNECTION", "SSH_CLIENT"} {
		cmd := exec.CommandContext(ctx, "tmux", "show-environment", key)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		line := strings.TrimSpace(string(out))
		if strings.HasPrefix(line, "-") {
			continue
		}
		if i := strings.IndexByte(line, '='); i >= 0 {
			line = line[i+1:]
		}
		if ip := sourceIPFromSSH(line); ip != "" {
			return ip
		}
	}
	return ""
}

func sourceIPFromSSH(v string) string {
	fields := strings.Fields(v)
	if len(fields) == 0 {
		return ""
	}
	if ip := net.ParseIP(fields[0]); ip != nil {
		return ip.String()
	}
	return ""
}

func parseWhois(data []byte) (device, nodeID string, err error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return "", "", fmt.Errorf("decode tailscale whois JSON: %w", err)
	}
	node := record(root["Node"])
	if node == nil {
		node = record(root["Machine"])
	}
	if node == nil {
		return "", "", errors.New("tailscale whois JSON missing Node/Machine")
	}

	device = firstString(node, "ComputedName", "Name", "HostName", "Hostname", "DNSName")
	if hostinfo := record(node["Hostinfo"]); device == "" && hostinfo != nil {
		device = firstString(hostinfo, "Hostname", "HostName")
	}
	if device == "" {
		return "", "", errors.New("tailscale whois JSON missing machine name")
	}

	nodeID = firstString(node, "StableID", "ID", "Id", "NodeID")
	if nodeID == "" {
		if v, ok := node["ID"].(float64); ok {
			nodeID = fmt.Sprintf("%.0f", v)
		}
	}
	return device, nodeID, nil
}

func record(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		switch v := m[key].(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return v
			}
		case float64:
			return fmt.Sprintf("%.0f", v)
		}
	}
	return ""
}

func ResourceAttributes(id Identity, existing string) string {
	kept := make([]string, 0, 8)
	for _, part := range strings.Split(existing, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key := part
		if i := strings.IndexByte(part, '='); i >= 0 {
			key = part[:i]
		}
		if strings.HasPrefix(key, "meter.") {
			continue
		}
		kept = append(kept, part)
	}
	kept = append(kept,
		"meter.person="+otelEscape(id.Person),
		"meter.device="+otelEscape(id.Device),
	)
	if id.NodeID != "" {
		kept = append(kept, "meter.node_id="+otelEscape(id.NodeID))
	}
	if id.SourceIP != "" {
		kept = append(kept, "meter.source_ip="+otelEscape(id.SourceIP))
	}
	return strings.Join(kept, ",")
}

func otelEscape(v string) string {
	var b strings.Builder
	const hex = "0123456789ABCDEF"
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c <= 0x20 || c >= 0x7f || strings.ContainsRune("\"',;\\=", rune(c)) {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0xf])
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
