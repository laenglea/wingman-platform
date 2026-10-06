package provider

import (
	"errors"
	"math"
	"net/http"
	"testing"
	"time"
)

func TestInvalidRequest(t *testing.T) {
	if InvalidRequest(nil) != nil {
		t.Fatal("nil must stay nil")
	}

	if got := CodeFromError(InvalidRequest(errors.New("bad")), http.StatusBadGateway); got != http.StatusBadRequest {
		t.Errorf("untyped error: got %d, want 400", got)
	}

	typed := &ProviderError{Code: http.StatusTooManyRequests}
	if got := InvalidRequest(typed); got != typed {
		t.Errorf("typed error must be returned unchanged, got %v", got)
	}
}

func TestRetryAfterHeaderValue(t *testing.T) {
	for _, tc := range []struct {
		delay time.Duration
		want  string
	}{
		{0, ""}, {-time.Second, ""}, {time.Nanosecond, "1"},
		{time.Second, "1"}, {1500 * time.Millisecond, "2"},
		{2 * time.Second, "2"}, {time.Duration(math.MaxInt64), "9223372037"},
	} {
		if got := RetryAfterHeaderValue(tc.delay); got != tc.want {
			t.Errorf("%v: got %q, want %q", tc.delay, got, tc.want)
		}
	}
}
