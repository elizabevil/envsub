package source

import (
	"github.com/elizabevil/envsub/internal/jsonx"
	"github.com/elizabevil/envsub/internal/values"
)

// JSON parses JSON values files. It implements values.Loader.
type JSON struct{}

// JSONName is the diagnostic name of the JSON values loader.
const JSONName = "json"

// Name identifies the loader in diagnostics.
func (JSON) Name() string { return JSONName }

// Load parses a JSON document into a value tree, preserving native types.
func (JSON) Load(data []byte) (*values.Tree, error) {
	root := map[string]any{}
	if err := jsonx.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	normalized, _ := values.NormalizeNumbers(root).(map[string]any)
	if normalized != nil {
		root = normalized
	}
	tree := values.NewTree()
	tree.Root = root
	tree.SeedOrigins()
	return tree, nil
}
