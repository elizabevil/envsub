// Package yamlx is the project's single YAML entry point, built on one library:
// go.yaml.in/yaml/v3.
//
// Decode walks a yaml.Node tree so integers stay integers, quoted scalars stay
// strings, duplicate mapping keys are rejected and merge keys are applied; the
// JSON round trip sigs.k8s.io/yaml uses for decoding would widen integers above
// 2^53 and lose the duplicate-key check.
//
// Marshal encodes the value tree with yaml.v3 as well. The rejected alternative
// — sigs.k8s.io/yaml, whose output keeps YAML sequences at the parent
// indentation — is kept runnable as a functional and performance comparison in
// test/yamlx; docs/06-review/06-07 records the size and style measurements.
package yamlx
