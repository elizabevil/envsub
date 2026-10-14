package source

import (
	"fmt"

	"github.com/elizabevil/envsub/internal/values"
	"github.com/elizabevil/envsub/internal/yamlx"
)

// YAML parses YAML values files. It implements values.Loader.
type YAML struct{}

// YAMLName is the diagnostic name of the YAML values loader.
const YAMLName = "yaml"

// Name identifies the loader in diagnostics.
func (YAML) Name() string { return YAMLName }

// Load parses a YAML document into a value tree, preserving native types.
//
// The codec lives in internal/yamlx, which decodes through yaml.v3 nodes so
// integers stay integers instead of being widened to float64 by a JSON round
// trip, resolves merge keys, and rejects duplicate keys.
func (YAML) Load(data []byte) (*values.Tree, error) {
	root, err := yamlx.Decode(data)
	if err != nil {
		return nil, err
	}
	if root == nil {
		root = map[string]any{}
	}
	object, ok := root.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("top-level YAML document must be a mapping, got %T", root)
	}
	tree := values.NewTree()
	tree.Root = object
	tree.SeedOrigins()
	return tree, nil
}
