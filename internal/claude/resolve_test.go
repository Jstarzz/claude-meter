package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePrefersNewestVersion(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "versions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "2.1.246")
	newer := filepath.Join(dir, "2.1.247")
	for _, path := range []string{old, newer} {
		if err := os.WriteFile(path, []byte("x"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Resolve(old, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != newer {
		t.Fatalf("got %q want %q", got, newer)
	}
}

func TestResolveSurvivesRemovedConfiguredVersion(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "versions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "2.1.246")
	newer := filepath.Join(dir, "2.1.250")
	if err := os.WriteFile(newer, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(missing, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != newer {
		t.Fatalf("got %q want %q", got, newer)
	}
}

func TestVersionParts(t *testing.T) {
	got := versionParts("2.10.3")
	if len(got) != 3 || got[0] != 2 || got[1] != 10 || got[2] != 3 {
		t.Fatalf("got %v", got)
	}
	if versionParts("latest") != nil {
		t.Fatal("expected non-version name to be rejected")
	}
}
