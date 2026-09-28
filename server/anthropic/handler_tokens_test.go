package anthropic

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/config"
	"github.com/adrianliechti/wingman/pkg/policy/noop"
)

func TestCountTokensResolvesModel(t *testing.T) {
	cfg := &config.Config{Policy: noop.New()}
	cfg.RegisterCompleter("test", &optionsCompleter{})
	h := New(cfg)

	for model, want := range map[string]int{"test": http.StatusOK, "missing": http.StatusNotFound} {
		rec := httptest.NewRecorder()
		body := `{"model":"` + model + `","messages":[{"role":"user","content":"hi"}]}`
		h.handleCountTokens(rec, httptest.NewRequest(http.MethodPost, "/messages/count_tokens", strings.NewReader(body)))

		if rec.Code != want {
			t.Errorf("%s: HTTP %d, want %d: %s", model, rec.Code, want, rec.Body.String())
		}
	}
}
