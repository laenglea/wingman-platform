package realtime

import (
	"encoding/json"
	"testing"
)

func TestToolSchemaNumbers(t *testing.T) {
	const schema = `{"enum":[9007199254740993,1.234567890123456789,1e400,null]}`
	tools, err := parseTools(json.RawMessage(`[{"type":"function","name":"lookup","parameters":` + schema + `}]`))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(tools[0].Parameters)
	if err != nil || string(data) != schema {
		t.Fatalf("schema changed: %s (%v)", data, err)
	}
}
