package provider

import (
	"errors"
	"net/http"
	"testing"
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
