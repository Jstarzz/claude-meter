package identity

import "testing"

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
	got := sourceIPFromSSH("100.108.157.80 53124 100.109.68.120 22")
	if got != "100.108.157.80" {
		t.Fatalf("got %q", got)
	}
}
