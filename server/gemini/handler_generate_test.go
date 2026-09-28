package gemini

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/config"
	"github.com/adrianliechti/wingman/pkg/policy/noop"
)

func TestGenerateContentUnknownModel(t *testing.T) {
	h := New(&config.Config{Policy: noop.New()})

	req := httptest.NewRequest(http.MethodPost, "/models/missing:generateContent", strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
	req.SetPathValue("model", "missing")

	rec := httptest.NewRecorder()
	h.handleGenerateContent(rec, req)

	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"NOT_FOUND"`) {
		t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
	}
}
