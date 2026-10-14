package schema

import (
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/elizabevil/envsub/internal/codec"
	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/values"
)

// printer renders jsonschema error kinds in English, matching the library's
// own default output.
var printer = message.NewPrinter(language.English)

// maxReportedCauses bounds how many validation failures are reported.
const maxReportedCauses = 8

// Validate checks the value tree against the schema.
func (s *Schema) Validate(tree *values.Tree) error {
	if s == nil || s.compiled == nil || tree == nil {
		return nil
	}
	// jsonschema/v6 understands json.Number but not codec.ExactInt; the tree is
	// only copied when it actually holds one (rules step 4).
	if err := s.compiled.Validate(codec.ToJSON(tree.Root)); err != nil {
		return diag.Wrap(diag.KindSchema, describeValidation(err))
	}
	return nil
}

// describeValidation renders a jsonschema validation failure with the location
// of the offending value.
func describeValidation(err error) error {
	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) {
		return err
	}
	causes := collectCauses(validationErr)
	if len(causes) == 0 {
		return fmt.Errorf("schema validation failed")
	}
	return fmt.Errorf("schema validation failed: %s", strings.Join(causes, "; "))
}

// collectCauses flattens the validation error tree into "at /path: reason"
// strings, using the library's default English printer.
func collectCauses(err *jsonschema.ValidationError) []string {
	var causes []string
	var walk func(node *jsonschema.ValidationError)
	walk = func(node *jsonschema.ValidationError) {
		if len(causes) >= maxReportedCauses {
			return
		}
		if len(node.Causes) == 0 {
			reason := ""
			if node.ErrorKind != nil {
				reason = node.ErrorKind.LocalizedString(printer)
			}
			causes = append(causes, fmt.Sprintf("at %s: %s", jsonPointer(node.InstanceLocation), reason))
			return
		}
		for _, cause := range node.Causes {
			walk(cause)
		}
	}
	walk(err)
	return causes
}

// jsonPointer renders an instance location as a JSON Pointer.
func jsonPointer(location []string) string {
	if len(location) == 0 {
		return pointerSeparator
	}
	return pointerSeparator + strings.Join(location, pointerSeparator)
}
