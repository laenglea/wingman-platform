package webtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/adrianliechti/wingman/test/harness"
)

func LiveHTTP(t *testing.T, dialect string) {
	t.Helper()
	local := LiveServer(t, "tavily")
	client := harness.NewClient()
	client.Timeout = 90 * time.Second
	search := map[string]any{"query": "Go programming language documentation"}
	extraction := map[string]any{"urls": []string{"https://go.dev/doc/", "https://wingman-e2e-nonexistent.invalid/"}}
	searchPath, extractPath := "/search", "/extract"
	key, referenceURL := os.Getenv("TAVILY_API_KEY"), "https://api.tavily.com"
	if dialect == "tavily" {
		search["max_results"] = 1
		search["include_domains"] = []string{"go.dev"}
	} else {
		search["numResults"] = 1
		search["includeDomains"] = []string{"go.dev"}
		extraction["text"] = true
		extractPath = "/contents"
		key = os.Getenv("EXA_API_KEY")
		referenceURL = "https://api.exa.ai"
	}
	post := func(ep harness.Endpoint, path string, body any) *harness.RawResponse {
		r, err := client.Post(t.Context(), ep, path, body)
		if err != nil {
			t.Fatal(err)
		}
		if r.StatusCode != http.StatusOK {
			t.Fatalf("%s HTTP %d: %s", ep.Name, r.StatusCode, r.RawBody)
		}
		return r
	}
	reference := harness.Endpoint{Name: "native-" + dialect, BaseURL: referenceURL, APIKey: key}
	nativeSearch := post(reference, searchPath, search)
	nativeExtract := post(reference, extractPath, extraction)
	assertSearch := func(t *testing.T, body map[string]any) {
		t.Helper()
		hits, ok := body["results"].([]any)
		if !ok || len(hits) != 1 {
			t.Fatal("search must return one ranked result")
		}
		hit := hits[0].(map[string]any)
		u, err := url.Parse(hit["url"].(string))
		if err != nil || u.Hostname() != "go.dev" {
			t.Fatal("domain restriction was not honored")
		}
		if title, ok := hit["title"].(string); !ok || title == "" {
			t.Fatal("missing title")
		}
		if dialect == "tavily" {
			if text, ok := hit["content"].(string); !ok || text == "" {
				t.Fatal("missing evidence")
			}
		}
	}
	assertSearch(t, nativeSearch.Body)
	if len(nativeExtract.Body["results"].([]any)) != 1 {
		t.Fatal("native extraction did not distinguish the missing page")
	}
	for _, backend := range []string{"tavily", "exa"} {
		t.Run(backend, func(t *testing.T) {
			endpoint := harness.Endpoint{Name: "wingman-" + backend, BaseURL: local.URL + "/v1/" + dialect, APIKey: LiveKey}
			search["model"] = backend
			extraction["model"] = backend
			actualSearch := post(endpoint, searchPath, search)
			assertSearch(t, actualSearch.Body)
			actualExtract := post(endpoint, extractPath, extraction)
			if len(actualExtract.Body["results"].([]any)) != 1 {
				t.Fatal("extraction did not distinguish the missing page")
			}
			if dialect == "tavily" {
				if len(actualExtract.Body["failed_results"].([]any)) != 1 {
					t.Fatal("missing partial failure")
				}
			} else {
				statuses := actualExtract.Body["statuses"].([]any)
				if len(statuses) != 2 || statuses[1].(map[string]any)["status"] != "error" {
					t.Fatal("missing contents status")
				}
			}
			compareContract(t, dialect, nativeSearch.Body, actualSearch.Body, false)
			compareContract(t, dialect, nativeExtract.Body, actualExtract.Body, true)
			t.Logf("%s API through %s backend: search, extraction and partial failure passed", dialect, backend)
		})
	}
	// The extraction API also works with the built-in HTTP backend.
	extraction["model"] = "fetch"
	actual := post(harness.Endpoint{Name: "wingman-fetch", BaseURL: local.URL + "/v1/" + dialect, APIKey: LiveKey}, extractPath, extraction)
	if len(actual.Body["results"].([]any)) != 1 {
		t.Fatal("built-in fetch backend failed")
	}
}

// Compare the portable response fields against live native responses. Optional
// vendor enrichment and ranking values are not part of this common contract.
func compareContract(t *testing.T, dialect string, reference, actual map[string]any, extraction bool) {
	t.Helper()
	project := func(body map[string]any) map[string]any {
		id := "requestId"
		if dialect == "tavily" {
			id = "request_id"
		}
		out := map[string]any{"requestId": body[id], "results": []any{}}
		fields := []string{"url", "title"}
		if dialect == "tavily" {
			if extraction {
				fields = []string{"url", "raw_content"}
			} else {
				fields = append(fields, "content")
			}
		} else if extraction {
			fields = []string{"url", "text"}
		}
		for _, value := range body["results"].([]any) {
			hit := value.(map[string]any)
			result := map[string]any{}
			for _, field := range fields {
				result[field] = hit[field]
			}
			out["results"] = append(out["results"].([]any), result)
		}
		return out
	}
	harness.CompareStructure(t, "native SDK contract", project(reference), project(actual), harness.CompareOption{Rules: map[string]harness.FieldRule{
		"requestId": harness.FieldNonEmpty, "results.*.url": harness.FieldNonEmpty,
		"results.*.title": harness.FieldType, "results.*.content": harness.FieldNonEmpty,
		"results.*.text": harness.FieldNonEmpty, "results.*.raw_content": harness.FieldNonEmpty,
	}})
}

func LiveSDK(t *testing.T, dialect string) {
	t.Helper()
	if os.Getenv("WEB_SDK_TEST") != "1" {
		t.Skip("set WEB_SDK_TEST=1 and install the official SDK test dependencies")
	}
	for _, backend := range []string{"tavily", "exa"} {
		t.Run(backend, func(t *testing.T) {
			local := LiveServer(t, backend)
			for _, language := range []string{"javascript", "python"} {
				t.Run(language, func(t *testing.T) { runSDK(t, local, dialect, language) })
			}
		})
	}
}

func runSDK(t *testing.T, local *httptest.Server, dialect, language string) {
	t.Helper()
	// Tests execute from server/{provider}; resolve the committed SDK probes.
	root, err := filepath.Abs("../../test/webtest/sdk")
	if err != nil {
		t.Fatal(err)
	}
	executable, script := "node", "clients.mjs"
	if language == "python" {
		executable = os.Getenv("WEB_TEST_PYTHON")
		if executable == "" {
			executable = "python3"
		}
		script = "clients.py"
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, filepath.Join(root, script))
	cmd.Env = append(os.Environ(), "WEB_TEST_URL="+local.URL, "WEB_TEST_KEY="+LiveKey, "WEB_TEST_DIALECT="+dialect)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s SDK failed: %s", language, output)
	}
	t.Logf("%s", output)
}
