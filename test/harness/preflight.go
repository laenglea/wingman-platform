package harness

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

var (
	configuredMu     sync.Mutex
	configuredModels = map[string]map[string]bool{}
)

// ConfiguredModels fetches the model list from a wingman endpoint once per
// base URL and caches it. Returns nil if the listing is unavailable.
func ConfiguredModels(baseURL, apiKey string) map[string]bool {
	configuredMu.Lock()
	defer configuredMu.Unlock()

	if models, ok := configuredModels[baseURL]; ok {
		return models
	}

	configuredModels[baseURL] = nil

	req, err := http.NewRequest("GET", strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return nil
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}

	models := map[string]bool{}

	for _, m := range payload.Data {
		models[m.ID] = true
	}

	for _, m := range payload.Models {
		models[strings.TrimPrefix(m.Name, "models/")] = true
	}

	if len(models) == 0 {
		return nil
	}

	configuredModels[baseURL] = models
	return models
}

// SkipUnlessConfigured skips the test when the wingman endpoint exposes a
// model listing and the given model is not part of it.
func SkipUnlessConfigured(t *testing.T, baseURL, apiKey, model string) {
	t.Helper()

	models := ConfiguredModels(baseURL, apiKey)

	if models == nil {
		return
	}

	if !models[model] {
		t.Skipf("model %q not configured in wingman", model)
	}
}
