package jsonutil

import (
	"encoding/json"
	"testing"
)

func TestUnmarshalPreservesNumbersAndValidatesEntireInput(t *testing.T) {
	const input = `{"id":9007199254740993,"values":[1.234567890123456789,1e400,null]}`
	var value any
	if err := Unmarshal([]byte(input), &value); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil || string(data) != input {
		t.Fatalf("round trip: %s (%v)", data, err)
	}
	for _, invalid := range []string{input + `{}`, input + ` garbage`, `{"a":1,}`} {
		var value any
		if err := Unmarshal([]byte(invalid), &value); err == nil || value != nil {
			t.Fatalf("accepted partial input %q: %v", invalid, value)
		}
	}
}
