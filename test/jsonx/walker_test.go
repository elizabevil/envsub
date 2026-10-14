// Package jsonx_test keeps the rejected JSON decoding alternative runnable so
// the design decision stays reproducible.
//
// Production decodes JSON with encoding/json/v2 (see internal/jsonx/jsonv2.go):
// a type-specific unmarshaler replaces v2's float64 default for values whose
// target type is any, and everything else is done by v2's Unmarshal. The
// alternative below is a single-pass token walker over the same jsontext
// decoder, which was the production implementation until the simplicity
// trade-off was chosen. It is faster (one decoder pass instead of re-entering
// Unmarshal per container) but it is a parsing layer of our own.
//
// The comparison is exercised by compare_test.go:
//
//	go test -v ./test/jsonx/                  # functional equivalence + timing report
//	go test -bench=. -benchmem ./test/jsonx/  # the same comparison as benchmarks
//
// Measurements live in docs/06-review/06-06-json-number-precision.md.
package jsonx_test

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
)

// walkerUnmarshalValue decodes a document into a tree with numbers as
// json.Number, using one pass over jsontext tokens.
func walkerUnmarshalValue(data []byte) (any, error) {
	decoder := jsontext.NewDecoder(bytes.NewReader(data))
	value, err := walkerDecodeValue(decoder)
	if err != nil {
		return nil, err
	}
	// Reject trailing content the way Unmarshal does.
	if offset := int(decoder.InputOffset()); offset < len(data) {
		if rest := bytes.TrimSpace(data[offset:]); len(rest) > 0 {
			return nil, fmt.Errorf("walker: unexpected trailing data %q", rest)
		}
	}
	return value, nil
}

// walkerDecodeValue reads one JSON value, preserving the exact number spelling.
func walkerDecodeValue(decoder *jsontext.Decoder) (any, error) {
	switch kind := decoder.PeekKind(); kind {
	case jsontext.KindNull:
		if _, err := decoder.ReadToken(); err != nil {
			return nil, err
		}
		return nil, nil
	case jsontext.KindTrue, jsontext.KindFalse:
		token, err := decoder.ReadToken()
		if err != nil {
			return nil, err
		}
		return token.Bool(), nil
	case jsontext.KindString:
		token, err := decoder.ReadToken()
		if err != nil {
			return nil, err
		}
		return token.String(), nil
	case jsontext.KindNumber:
		raw, err := decoder.ReadValue()
		if err != nil {
			return nil, err
		}
		return json.Number(string(raw)), nil
	case jsontext.KindBeginObject:
		return walkerDecodeObject(decoder)
	case jsontext.KindBeginArray:
		return walkerDecodeArray(decoder)
	default:
		return nil, fmt.Errorf("walker: unexpected token %s", kind)
	}
}

// walkerDecodeObject reads a JSON object into a map.
func walkerDecodeObject(decoder *jsontext.Decoder) (map[string]any, error) {
	if _, err := decoder.ReadToken(); err != nil {
		return nil, err
	}
	object := map[string]any{}
	for decoder.PeekKind() != jsontext.KindEndObject {
		key, err := decoder.ReadToken()
		if err != nil {
			return nil, err
		}
		// A token is voided by the next decoder call, so read the key now.
		name := key.String()
		value, err := walkerDecodeValue(decoder)
		if err != nil {
			return nil, err
		}
		object[name] = value
	}
	if _, err := decoder.ReadToken(); err != nil {
		return nil, err
	}
	return object, nil
}

// walkerDecodeArray reads a JSON array into a slice.
func walkerDecodeArray(decoder *jsontext.Decoder) ([]any, error) {
	if _, err := decoder.ReadToken(); err != nil {
		return nil, err
	}
	array := []any{}
	for decoder.PeekKind() != jsontext.KindEndArray {
		value, err := walkerDecodeValue(decoder)
		if err != nil {
			return nil, err
		}
		array = append(array, value)
	}
	if _, err := decoder.ReadToken(); err != nil {
		return nil, err
	}
	return array, nil
}
