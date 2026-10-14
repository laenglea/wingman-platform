package jsonutil

import (
	"encoding/json"
	"testing"
)

func TestExactIntegerNumbers(t *testing.T) {
	for _, tc := range []struct {
		number string
		want   int64
	}{
		{"0", 0}, {"-0.0e999999999999999999999", 0}, {"3.0", 3}, {"30e-1", 3},
		{"9007199254740993.0", 9007199254740993}, {"90071992547409930e-1", 9007199254740993},
		{"9223372036854775807", 9223372036854775807}, {"-9223372036854775808", -9223372036854775808},
	} {
		got, ok := Int64(json.Number(tc.number))
		if !ok || got != tc.want || !IsInteger(json.Number(tc.number)) {
			t.Errorf("%s: %d, %v", tc.number, got, ok)
		}
	}
	for _, n := range []json.Number{"1e400", "1e999999999999999999999", "9223372036854775808"} {
		if !IsInteger(n) {
			t.Errorf("%s should be integral", n)
		}
		if _, ok := Int64(n); ok {
			t.Errorf("%s should overflow", n)
		}
	}
	for _, n := range []json.Number{"1.0000000000000000001", "1e-400", "1e-999999999999999999999", "true", "null", "2/1", "01", "", "1.1"} {
		if IsInteger(n) {
			t.Errorf("%s should not be integral", n)
		}
		if _, ok := Int64(n); ok {
			t.Errorf("%s should not convert", n)
		}
	}
}
