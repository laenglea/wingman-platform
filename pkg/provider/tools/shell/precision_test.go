package shell

import (
	"encoding/json"
	"testing"
)

func TestNativeArgumentsPreserveNumbers(t *testing.T) {
	for _, tc := range []struct {
		convert func(string) map[string]any
		input   string
	}{
		{BashInput, `{"command":"pwd","limit":9007199254740993}`},
		{ShellAction, `{"commands":["pwd"],"timeout_ms":9007199254740993}`},
		{LocalShellAction, `{"command":["pwd"],"timeout_ms":9007199254740993,"type":"exec"}`},
	} {
		data, err := json.Marshal(tc.convert(tc.input))
		if err != nil || string(data) != tc.input {
			t.Fatalf("arguments changed: %s (%v)", data, err)
		}
	}
}
