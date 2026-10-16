package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/config"
	"github.com/adrianliechti/wingman/pkg/policy"
	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/provider/adapter/decider"
	"github.com/adrianliechti/wingman/server"
)

type nativeDecisionBackend struct{ calls int }

func (p *nativeDecisionBackend) Decide(_ context.Context, input *provider.DecisionInput) (*provider.Decision, error) {
	p.calls++
	return &provider.Decision{
		Model: "native-model",
		Answers: []provider.DecisionAnswer{{
			ID: input.Questions[0].ID, Noul: &provider.NoulAnswer{Probability: 1},
		}},
	}, nil
}

func TestDecisionSelectionAndAccess(t *testing.T) {
	for _, endpoint := range []struct{ path, input string }{
		{"/v1/systemone", `"state":"hello","questions":{"q":{"type":"noul"}}`},
		{"/v1/decisions", `"input":"hello","questions":[{"type":"predicate","instructions":""}]`},
	} {
		for _, tt := range []struct {
			name, modelField, wantAlias, wantModel string
			native, adapter, denied                bool
			status                                 int
		}{
			{name: "native default", native: true, adapter: true, wantAlias: "z-native", wantModel: "native-model", status: 200},
			{name: "empty native default", modelField: `"model":"",`, native: true, wantAlias: "z-native", wantModel: "native-model", status: 200},
			{name: "adapter default", adapter: true, wantAlias: "gpt-6-luna", wantModel: "actual-model", status: 200},
			{name: "explicit adapter", modelField: `"model":"gpt-6-luna",`, native: true, adapter: true, wantAlias: "gpt-6-luna", wantModel: "actual-model", status: 200},
			{name: "explicit embedding alias", modelField: `"model":"vectors",`, native: true, adapter: true, wantAlias: "vectors", wantModel: "embedding-model", status: 200},
			{name: "explicit second native", modelField: `"model":"a-native",`, native: true, wantAlias: "a-native", wantModel: "native-model", status: 200},
			{name: "disabled chat", modelField: `"model":"gpt-6-luna",`, native: true, status: 400},
			{name: "disabled embedding", modelField: `"model":"embedding",`, adapter: true, status: 400},
			{name: "unknown model", modelField: `"model":"missing",`, native: true, adapter: true, status: 400},
			{name: "no decision models", status: 400},
			{name: "denied default", native: true, adapter: true, denied: true, wantAlias: "z-native", status: 404},
			{name: "denied adapter default", adapter: true, denied: true, wantAlias: "gpt-6-luna", status: 404},
			{name: "denied embedding alias", modelField: `"model":"vectors",`, adapter: true, denied: true, wantAlias: "vectors", status: 404},
		} {
			t.Run(endpoint.path+"/"+tt.name, func(t *testing.T) {
				access := &decisionPolicy{}
				if tt.denied {
					access.err = policy.ErrAccessDenied
				}
				cfg := &config.Config{Policy: access}
				chat := &decisionCompleter{output: `{"q0":{"0":1,"1":0}}`}
				cfg.RegisterCompleter("gpt-6-luna", chat)
				cfg.RegisterEmbedder("embedding", decisionEmbedder{})
				if tt.adapter {
					cfg.RegisterDecisionAdapter("gpt-6-luna", decider.FromCompleter("gpt-6-luna", chat))
					cfg.RegisterDecisionAdapter("vectors", decider.FromEmbedder("embedding", decisionEmbedder{}))
				}
				first, second := &nativeDecisionBackend{}, &nativeDecisionBackend{}
				if tt.native {
					cfg.RegisterDecider("z-native", first)
					cfg.RegisterDecider("a-native", second)
				}
				s, err := server.New(cfg)
				if err != nil {
					t.Fatal(err)
				}
				w := httptest.NewRecorder()
				body := "{" + tt.modelField + endpoint.input + "}"
				s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, endpoint.path, strings.NewReader(body)))
				if w.Code != tt.status {
					t.Fatalf("status = %d, want %d: %s", w.Code, tt.status, w.Body.String())
				}
				if tt.wantAlias != "" && access.seen != "model/"+tt.wantAlias+"/access" {
					t.Fatalf("policy checked %q, want alias %q", access.seen, tt.wantAlias)
				}
				if tt.status == 200 && !strings.Contains(w.Body.String(), `"model":"`+tt.wantModel+`"`) {
					t.Fatalf("wrong model response: %s", w.Body.String())
				}
				if tt.status != 200 && first.calls+second.calls+chat.calls != 0 {
					t.Fatal("rejected request reached a backend")
				}
				if tt.status == 200 && tt.wantAlias == "z-native" && (first.calls != 1 || second.calls+chat.calls != 0) {
					t.Fatal("default did not select the first native decider")
				}
			})
		}
	}
}
