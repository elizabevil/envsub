// Package jsonx is the project's single JSON entry point.
//
// Two interchangeable implementations are selected at build time:
//
//   - default (no build tag): encoding/json/v2 from the standard library. It
//     keeps the binary small (about 41% smaller than the sonic build).
//   - -tags=sonic: github.com/bytedance/sonic. It decodes several times faster
//     at the cost of a much larger binary (it links its own JIT decoder and a
//     multi-architecture assembler).
//
// Both implementations decode numbers into any as json.Number, so integers keep
// their exact spelling, and both encode json.Number back as a JSON number.
package jsonx

// errorPrefix names this package in error messages.
const errorPrefix = "jsonx: "
