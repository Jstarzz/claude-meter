package claude

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var blocked = []string{
	"/usr/local/bin/claude",
	"/usr/local/lib/claude-meter/claude-meter",
}

func Resolve(configured, fallback string) (string, error) {
	seen := map[string]bool{}
	candidates := make([]string, 0, 16)
	for _, path := range []string{configured, fallback} {
		if dir := versionDir(path); dir != "" {
			for _, candidate := range versionCandidates(dir) {
				addCandidate(&candidates, seen, candidate)
			}
	}
	}
	addCandidate(&candidates, seen, configured)
	addCandidate(&candidates, seen, fallback)
	addCandidate(&candidates, seen, "/usr/local/lib/claude-meter/claude-real")
	addCandidate(&candidates, seen, "/usr/local/lib/claude-meter/claude-recovered")

	for _, candidate := range candidates {
		if valid(candidate) {
			resolved, err := filepath.EvalSymlinks(candidate)
			if err == nil {
				return resolved, nil
			}
			return filepath.Clean(candidate), nil
		}
	}
	return "", errors.New("real Claude binary unavailable")
}

func addCandidate(out *[]string, seen map[string]bool, path string) {
	path = strings.TrimSpace(path)
	if path == "" || seen[path] {
		return
	}
	seen[path] = true
	*out = append(*out, path)
}

func versionDir(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	dir := filepath.Dir(path)
	if filepath.Base(dir) != "versions" {
		return ""
	}
	return dir
}

func versionCandidates(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type item struct {
		path  string
		name  string
		parts []int
	}
	items := make([]item, 0, len(entries))
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if !valid(path) {
			continue
		}
		items = append(items, item{path: path, name: entry.Name(), parts: versionParts(entry.Name())})
	}
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if len(a.parts) > 0 && len(b.parts) > 0 {
			for n := 0; n < len(a.parts) || n < len(b.parts); n++ {
				av, bv := 0, 0
				if n < len(a.parts) {
					av = a.parts[n]
				}
				if n < len(b.parts) {
					bv = b.parts[n]
				}
				if av != bv {
					return av > bv
				}
			}
		}
		return a.name > b.name
	})
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.path)
	}
	return out
}

func versionParts(name string) []int {
	fields := strings.Split(name, ".")
	if len(fields) == 0 {
		return nil
	}
	out := make([]int, 0, len(fields))
	for _, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}

func valid(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	for _, denied := range blocked {
		deniedResolved, err := filepath.EvalSymlinks(denied)
		if err != nil {
			deniedResolved = filepath.Clean(denied)
		}
		if resolved == deniedResolved || filepath.Clean(resolved) == filepath.Clean(denied) {
			return false
		}
	}
	info, err := os.Stat(resolved)
	return err == nil && !info.IsDir() && info.Mode().Perm()&0111 != 0
}
