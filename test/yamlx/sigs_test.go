// Package yamlx_test keeps the rejected YAML encoder runnable so the choice
// stays reproducible.
//
// Production encodes YAML with go.yaml.in/yaml/v3 (internal/yamlx/marshal.go):
// a single YAML library, exact numbers and null for typed nil collections. This
// package compares that encoder with sigs.k8s.io/yaml, which renders YAML
// through an encoding/json round trip and keeps sequences at the parent
// indentation; it was the production encoder until 2026-10-08.
//
// sigs.k8s.io/yaml is a test-only dependency here: it is not linked into the
// binary, which can be checked with
//
//	go list -deps ./cmd/envsub | grep -c 'sigs.k8s.io/yaml'   # 0
//
// Measurements and the decision are in docs/06-review/06-07.
//
//	go test -v ./test/yamlx/                  # functional equivalence + timing
//	go test -bench=. -benchmem ./test/yamlx/  # the same comparison as benchmarks
package yamlx_test

import sigs "sigs.k8s.io/yaml"

// sigsMarshal is the alternative encoder under comparison.
func sigsMarshal(value any) ([]byte, error) {
	return sigs.Marshal(value)
}
