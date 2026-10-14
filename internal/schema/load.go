package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/logging"
	"github.com/elizabevil/envsub/internal/values"
)

// Load normalizes, inspects and compiles a schema document.
//
// raw must be a JSON-compatible data tree (maps and slices), as produced by
// the source package. source is only used in diagnostics.
func Load(ctx context.Context, raw any, source string, policy UnknownPolicy) (*Schema, error) {
	if raw == nil {
		return nil, nil
	}
	normalized := values.CloneValue(raw)
	unknown, err := normalizeSchema(normalized, "")
	if err != nil {
		return nil, diag.Wrap(diag.KindSchema, fmt.Errorf("normalize schema %s: %w", source, err))
	}
	if err := reportUnknown(ctx, unknown, policy); err != nil {
		return nil, err
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaResourceName, normalized); err != nil {
		return nil, diag.Wrap(diag.KindSchema, fmt.Errorf("load schema %s: %w", source, err))
	}
	compiled, err := compiler.Compile(schemaResourceName)
	if err != nil {
		return nil, diag.Wrap(diag.KindSchema, fmt.Errorf("compile schema %s: %w", source, err))
	}
	schema := &Schema{Raw: normalized, Source: source, compiled: compiled}
	schema.walk = expandSchema(schema.Raw)
	return schema, nil
}

// reportUnknown applies the unknown keyword policy.
func reportUnknown(ctx context.Context, unknown []string, policy UnknownPolicy) error {
	if len(unknown) == 0 {
		return nil
	}
	switch policy {
	case UnknownWarn:
		logger := logging.FromContext(ctx)
		for _, keyword := range unknown {
			logger.Warn("unknown schema keyword", "keyword", keyword)
		}
		return nil
	case UnknownError:
		return diag.New(diag.KindSchema,
			"unknown schema keywords: %s", strings.Join(unknown, ", "))
	default:
		return nil
	}
}
