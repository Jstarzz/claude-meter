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
	sourceIP := sourceIPFromSSH(os.Getenv("SSH_CONNECTION"))
	if sourceIP == "" {
		if r.Config.LocalPerson != "" {
			return Identity{Person: r.Config.LocalPerson, Device: "local"}, nil
		}
		return Identity{}, errors.New("SSH_CONNECTION is empty and local_person is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.Config.TailscaleBin, "whois", "--json", sourceIP)
	out, err := cmd.Output()
	if err != nil {
		return Identity{}, fmt.Errorf("tailscale whois %s: %w", sourceIP, err)
	}

	device, nodeID, err := parseWhois(out)
	if err != nil {
		return Identity{}, err
	}
	normalized := config.NormalizeDevice(device)
	person, ok := r.Config.Devices[normalized]
	if !ok {
		return Identity{}, fmt.Errorf("unmapped Tailscale device %q (source %s)", device, sourceIP)
	}
	return Identity{Person: person, Device: normalized, NodeID: nodeID, SourceIP: sourceIP}, nil
}

func sourceIPFromSSH(v string) string {
	fields := strings.Fields(v)
	if len(fields) < 4 {
		return ""
	}
	host := fields[0]
	if ip := net.ParseIP(host); ip != nil {
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
