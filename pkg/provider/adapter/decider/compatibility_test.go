package decider

import (
	"context"
	"iter"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestCompleterHistoryAssetsAndCustomScores(t *testing.T) {
	cache := &provider.CacheOptions{Key: "tenant", Mode: provider.CacheModeExplicit}
	image := &provider.File{Content: []byte("hi"), ContentType: "image/png"}
	backend := completerFunc(func(ctx context.Context, messages []provider.Message, opts *provider.CompleteOptions) iter.Seq2[*provider.Completion, error] {
		if opts.CacheOptions != cache || len(messages) != 5 || messages[1].Content[0].CacheControl == nil || messages[2].Content[0].ToolResult == nil || messages[3].Content[1].File != image || !strings.Contains(messages[4].Text(), "severity") {
			t.Errorf("lost evidence/options: %+v %+v", messages, opts)
		}
		return func(yield func(*provider.Completion, error) bool) {
			msg := provider.AssistantMessage(`{"q0":{"0":0.25,"1":0.75}}`)
			yield(&provider.Completion{Message: &msg}, nil)
		}
	})
	input := &provider.DecisionInput{Messages: []provider.Message{
		{Role: provider.MessageRoleUser, Content: []provider.Content{{Text: "reference", CacheControl: &provider.CacheControl{}}}},
		provider.ToolMessage("f", "evidence"),
	}, Assets: map[string]*provider.File{"photo": image}, CacheOptions: cache, Questions: []provider.DecisionQuestion{{ID: "s", Instructions: []provider.Content{provider.TextContent("severity"), provider.FileContent(image)}, Score: &provider.ScoreQuestion{Levels: []any{"low", "high"}, Labels: []string{"Low", "High"}, Values: []int{-5, 15}}}}}
	result, err := FromCompleter("m", backend).Decide(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Answers[0].Score.Score != 10 || result.Answers[0].Score.Levels[1].Value != 15 || result.Answers[0].Score.Levels[1].Label != "High" {
		t.Fatalf("lost custom score: %+v", result.Answers[0].Score)
	}
}

func TestCompleterValidatesBoxOutput(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"box":{"top":0,"bottom":1,"left":0,"right":1},"confidence":null}`, true},
		{`{"box":{"top":0.1,"bottom":0.9,"left":0.2,"right":0.8},"confidence":0}`, true},
		{`{"box":{"top":0,"bottom":1,"left":0,"right":1}}`, false},
		{`null`, false}, {`[]`, false}, {`{}`, false},
		{`{"box":{"top":0,"bottom":1,"left":0},"confidence":null}`, false},
		{`{"box":{"top":1,"bottom":0,"left":0,"right":1},"confidence":0.5}`, false},
		{`{"box":{"top":0,"bottom":1,"left":0,"right":2},"confidence":0.5}`, false},
		{`{"box":{"top":0,"bottom":1,"left":0,"right":1},"confidence":-1}`, false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			backend := completerFunc(func(context.Context, []provider.Message, *provider.CompleteOptions) iter.Seq2[*provider.Completion, error] {
				return func(yield func(*provider.Completion, error) bool) {
					msg := provider.AssistantMessage(`{"q0":` + tc.body + `}`)
					yield(&provider.Completion{Message: &msg}, nil)
				}
			})
			result, err := FromCompleter("m", backend).Decide(context.Background(), &provider.DecisionInput{State: "photo", Questions: []provider.DecisionQuestion{{Box: &provider.BoxQuestion{}}}})
			if (err == nil) != tc.valid {
				t.Fatalf("result %+v, error %v", result, err)
			}
		})
	}
}

func TestEmbedderRejectsVisionAndBoxInputs(t *testing.T) {
	image := &provider.File{Content: []byte("hi"), ContentType: "image/png"}
	backend := embedderFunc(func(context.Context, []string, *provider.EmbedOptions) (*provider.Embedding, error) {
		t.Error("unsupported request reached embedder")
		return nil, nil
	})
	for name, input := range map[string]*provider.DecisionInput{
		"assets":            {State: "photo", Assets: map[string]*provider.File{"photo": image}, Questions: []provider.DecisionQuestion{{Noul: &provider.NoulQuestion{}}}},
		"question image":    {Questions: []provider.DecisionQuestion{{Instructions: []provider.Content{provider.FileContent(image)}, Noul: &provider.NoulQuestion{}}}},
		"input image":       {Messages: []provider.Message{{Role: provider.MessageRoleUser, Content: []provider.Content{provider.FileContent(image)}}}, Questions: []provider.DecisionQuestion{{Noul: &provider.NoulQuestion{}}}},
		"tool output image": {Messages: []provider.Message{{Role: provider.MessageRoleUser, Content: []provider.Content{provider.ToolResultContent(provider.ToolResult{ID: "f", Parts: []provider.Part{{File: image}}})}}}, Questions: []provider.DecisionQuestion{{Noul: &provider.NoulQuestion{}}}},
		"box":               {State: "x", Questions: []provider.DecisionQuestion{{Box: &provider.BoxQuestion{}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := FromEmbedder("m", backend).Decide(context.Background(), input); provider.CodeFromError(err, 0) != 400 {
				t.Fatalf("expected unsupported input: %v", err)
			}
		})
	}
}

func TestEmbedderHistoryAndSingleLevelScore(t *testing.T) {
	backend := embedderFunc(func(ctx context.Context, texts []string, opts *provider.EmbedOptions) (*provider.Embedding, error) {
		found := false
		for _, text := range texts {
			if strings.Contains(text, "lookup({})") && strings.Contains(text, "result") && strings.Contains(text, "refused") {
				found = true
			}
		}
		if !found {
			t.Fatalf("conversation lost: %v", texts)
		}
		vectors := make([][]float32, len(texts))
		for i := range vectors {
			vectors[i] = []float32{1, 0}
		}
		return &provider.Embedding{Embeddings: vectors}, nil
	})
	input := &provider.DecisionInput{Messages: []provider.Message{{Role: provider.MessageRoleAssistant, Content: []provider.Content{provider.RefusalContent("refused"), provider.ToolCallContent(provider.ToolCall{Name: "lookup", Arguments: "{}"})}}, provider.ToolMessage("f", "result")}, Questions: []provider.DecisionQuestion{{Instructions: []provider.Content{provider.TextContent("rate")}, Score: &provider.ScoreQuestion{Levels: []any{"only"}}}}}
	result, err := FromEmbedder("m", backend).Decide(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Answers[0].Score.Score != 0 || result.Answers[0].Score.Confidence != 1 || result.Answers[0].Score.Levels[0].Probability != 1 {
		t.Fatalf("bad single-level result: %+v", result.Answers[0].Score)
	}
}
