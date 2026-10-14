//go:build !sonic

package jsonx

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"github.com/elizabevil/envsub/internal/codec"
)

// Unmarshal decodes data into target.
//
// Tree-shaped targets (any, map[string]any, []any) are decoded by
// encoding/json/v2 with a type-specific unmarshaler installed. v2 has no
// UseNumber switch — it represents a JSON number as float64 when the target
// type is any, which loses precision above 2^53 — and WithUnmarshalers is the
// extension point its documentation points at. The unmarshaler only replaces
// that default: numbers become json.Number, everything else is decoded by v2
// itself. Concrete Go types are already exact in v2 and go straight to it.
//
// See docs/06-review/06-06-json-number-precision.md for the measurements; the
// alternative single-pass token walker is kept runnable in test/jsonx.
func Unmarshal(data []byte, target any) error {
	switch target.(type) {
	case *any, *map[string]any, *[]any:
		return tree.unmarshal(data, target)
	default:
		return jsonv2.Unmarshal(data, target)
	}
}

// UnmarshalValue decodes any JSON document into a JSON-compatible tree with
// numbers preserved as json.Number.
func UnmarshalValue(data []byte) (any, error) {
	var value any
	if err := Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

// treeDecoder decodes values whose target type is any into a JSON-compatible
// tree with numbers preserved as json.Number.
//
// The unmarshaler is built once and held by the decoder. It cannot be a
// package-level *jsonv2.Unmarshalers variable: the hook and the container
// decoding it delegates to reference each other, which Go rejects as an
// initialization cycle (hook -> decode -> unmarshal -> hook). Going through the
// decoder's own field breaks that cycle without an init function, and building
// the hook per call measurably slows decoding down.
type treeDecoder struct {
	hook *jsonv2.Unmarshalers
}

// tree is the decoder used by Unmarshal for the tree-shaped targets.
var tree = newTreeDecoder()

func newTreeDecoder() *treeDecoder {
	decoder := &treeDecoder{}
	decoder.hook = jsonv2.UnmarshalFunc[*any](decoder.decode)
	return decoder
}

// unmarshal decodes data into one of the tree targets with the hook installed.
func (d *treeDecoder) unmarshal(data []byte, target any) error {
	return jsonv2.Unmarshal(data, target, jsonv2.WithUnmarshalers(d.hook))
}

// decode maps one raw JSON value onto its Go representation. Containers are
// decoded by v2 itself (through unmarshal, so nested values visit the hook too);
// scalars need no further parsing because v2 only hands out valid values.
func (d *treeDecoder) decode(raw []byte, target *any) error {
	trimmed := bytes.TrimSpace(raw)
	switch jsontext.Value(trimmed).Kind() {
	case jsontext.KindBeginObject:
		var object map[string]any
		if err := d.unmarshal(trimmed, &object); err != nil {
			return err
		}
		*target = object
	case jsontext.KindBeginArray:
		var array []any
		if err := d.unmarshal(trimmed, &array); err != nil {
			return err
		}
		*target = array
	case jsontext.KindString:
		var text string
		if err := jsonv2.Unmarshal(trimmed, &text); err != nil {
			return err
		}
		*target = text
	case jsontext.KindTrue, jsontext.KindFalse:
		var flag bool
		if err := jsonv2.Unmarshal(trimmed, &flag); err != nil {
			return err
		}
		*target = flag
	case jsontext.KindNull:
		var nothing any
		if err := jsonv2.Unmarshal(trimmed, &nothing); err != nil {
			return err
		}
		*target = nothing
	case jsontext.KindNumber:
		*target = json.Number(string(trimmed))
	default:
		return fmt.Errorf(errorPrefix+"unexpected JSON value %q", trimmed)
	}
	return nil
}

// Marshal encodes value as compact JSON.
func Marshal(value any) ([]byte, error) {
	if err := codec.RejectNonFinite(value); err != nil {
		return nil, err
	}
	return jsonv2.Marshal(value, marshalOptions()...)
}

// MarshalIndent encodes value as two-space indented JSON.
func MarshalIndent(value any) ([]byte, error) {
	if err := codec.RejectNonFinite(value); err != nil {
		return nil, err
	}
	options := append(marshalOptions(), jsontext.WithIndent("  "))
	return jsonv2.Marshal(value, options...)
}

// marshalOptions keeps the output deterministic: Deterministic sorts object
// keys, because encoding/json/v2 does not sort them by default and toJson must
// not depend on Go's random map iteration order.
//
// Typed nil maps and slices use the library default ("{}" and "[]") per the
// serialization rules (03-12 V1); this only sorts keys.
func marshalOptions() []jsonv2.Options {
	return []jsonv2.Options{jsonv2.Deterministic(true)}
}
