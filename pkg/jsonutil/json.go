// Package jsonutil provides lossless decoding for opaque JSON carried between
// clients, providers and tools. Typed protocol fields keep their normal types.
package jsonutil

import (
	"bytes"
	"encoding/json"
)

// Unmarshal works like encoding/json.Unmarshal, but numbers in interface
// values become json.Number rather than float64. Custom UnmarshalJSON methods
// still control their own decoding and must preserve numbers themselves.
func Unmarshal(data []byte, value any) error {
	// Validate the entire input before populating value, just as Unmarshal
	// does. A Decoder alone would accept the first of two concatenated values.
	if !json.Valid(data) {
		return json.Unmarshal(data, value)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(value)
}
