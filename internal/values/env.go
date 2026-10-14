// Package values holds the merged value tree, path resolution and origins.
package values

import (
	"os"
	"strings"
)

// EnvFromOS snapshots the process environment into a string map.
func EnvFromOS() map[string]string {
	env := map[string]string{}
	for _, kv := range os.Environ() {
		key, value, ok := strings.Cut(kv, assignmentSeparator)
		if !ok {
			continue
		}
		env[key] = value
	}
	return env
}

// EnvFromOSFiltered snapshots the process environment keeping only names that
// start with prefix; an empty prefix keeps every variable. Names are kept as-is
// (the prefix is not stripped), so the naming rules for lookups do not change.
func EnvFromOSFiltered(prefix string) map[string]string {
	env := EnvFromOS()
	if prefix == "" {
		return env
	}
	filtered := make(map[string]string, len(env))
	for key, value := range env {
		if strings.HasPrefix(key, prefix) {
			filtered[key] = value
		}
	}
	return filtered
}
