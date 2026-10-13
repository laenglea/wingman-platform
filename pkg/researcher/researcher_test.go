package researcher

import (
	"errors"
	"iter"
	"testing"
)

func TestCollect(t *testing.T) {
	failure := errors.New("research failed")
	for _, tt := range []struct {
		name    string
		results []*Result
		err     error
		wantErr bool
	}{
		{"progress and final", []*Result{nil, {Progress: "Searching"}, {Content: "report"}}, nil, false},
		{"empty report", []*Result{{Content: ""}}, nil, false},
		{"missing final", []*Result{{Progress: "Searching"}}, nil, true},
		{"error after progress", []*Result{{Progress: "Searching"}}, failure, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stream := iter.Seq2[*Result, error](func(yield func(*Result, error) bool) {
				for _, result := range tt.results {
					if !yield(result, nil) {
						return
					}
				}
				if tt.err != nil {
					yield(nil, tt.err)
				}
			})
			result, err := Collect(stream)
			if (err != nil) != tt.wantErr || (tt.err != nil && !errors.Is(err, tt.err)) {
				t.Fatalf("result = %+v, error = %v", result, err)
			}
			if !tt.wantErr && result != tt.results[len(tt.results)-1] {
				t.Fatalf("did not preserve final report: %+v", result)
			}
		})
	}
}
