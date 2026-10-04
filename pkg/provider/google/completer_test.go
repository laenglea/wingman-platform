package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
	"google.golang.org/genai/interactions/models/interactions"
)

func TestComplete_InteractionRequest(t *testing.T) {
	var request interactions.CreateModelInteraction
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1beta/interactions" {
			t.Errorf("unexpected endpoint: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "test-token" {
			t.Error("missing authentication")
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		interactionEvents(createdEvent, completedEvent)(w, r)
	}
	c, err := NewCompleter("models/gemini-3.8-flash", WithToken("test-token"), WithClient(newTestClient(t, handler)))
	if err != nil {
		t.Fatal(err)
	}
	messages := []provider.Message{
		provider.SystemMessage("You are helpful."), provider.SystemMessage("Be brief."),
		provider.UserMessage("question"),
		{Role: provider.MessageRoleAssistant, Content: []provider.Content{
			provider.ReasoningContent(provider.Reasoning{Summary: "thinking", Signature: "Ev8BAA=="}),
			provider.ToolCallContent(provider.ToolCall{ID: "c1", Name: "lookup", Namespace: "weather", Arguments: `{"city":"Zurich"}`}),
		}},
		{Role: provider.MessageRoleUser, Content: []provider.Content{provider.ToolResultContent(provider.ToolResult{
			ID: "c1", IsError: true, Parts: []provider.Part{{Text: `{"error":"unavailable"}`}, {File: &provider.File{Content: []byte("image"), ContentType: "image/png"}}},
		})}},
		{Role: provider.MessageRoleUser, Content: []provider.Content{provider.FileContent(&provider.File{Content: []byte("pdf"), ContentType: "application/pdf"})}},
	}
	options := &provider.CompleteOptions{
		MaxTokens: new(1024), Stop: []string{"STOP"},
		Tools:            []provider.Tool{{Name: "weather", Tools: []provider.Tool{{Name: "lookup", Strict: new(true), Parameters: map[string]any{"type": "object"}}}}},
		ReasoningOptions: &provider.ReasoningOptions{Effort: provider.EffortXHigh, IncludeSummary: true},
		Schema:           &provider.Schema{Properties: map[string]any{"type": "object", "properties": map[string]any{"answer": map[string]any{"type": "string"}}}},
	}
	for _, err := range c.Complete(context.Background(), messages, options) {
		if err != nil {
			t.Fatal(err)
		}
	}
	if request.Store == nil || *request.Store || request.Stream == nil || !*request.Stream || request.PreviousInteractionID != nil {
		t.Fatalf("request is not stateless: %+v", request)
	}
	if request.Model != "gemini-3.8-flash" || value(request.SystemInstruction) != "You are helpful.\n\nBe brief." {
		t.Fatalf("incorrect model/instructions: %+v", request)
	}
	config := request.GenerationConfig
	if value(config.MaxOutputTokens) != 1024 || !reflect.DeepEqual(config.StopSequences, []string{"STOP"}) || value(config.ThinkingLevel) != interactions.ThinkingLevelHigh || value(config.ThinkingSummaries) != interactions.ThinkingSummariesAuto {
		t.Fatalf("configuration lost: %+v", config)
	}
	if config.Temperature != nil || config.TopP != nil {
		t.Fatal("Gemini sampling parameters must be omitted")
	}
	if config.ToolChoice == nil || value(config.ToolChoice.ToolChoiceType) != interactions.ToolChoiceTypeValidated || len(request.Tools) != 1 || value(request.Tools[0].Function.Name) != "weather_lookup" {
		t.Fatalf("tools lost: %+v", request.Tools)
	}
	if request.ResponseFormat == nil || request.ResponseFormat.ResponseFormat.TextResponseFormat.Schema["type"] != "object" || value(request.ResponseFormat.ResponseFormat.TextResponseFormat.MimeType) != "application/json" {
		t.Fatalf("schema lost: %+v", request.ResponseFormat)
	}
	steps := request.Input.ArrayOfStep
	if len(steps) != 5 || steps[0].UserInputStep.Content[0].TextContent.Text != "question" || value(steps[1].ThoughtStep.Signature) != "Ev8BAA==" || steps[2].FunctionCallStep.Name != "weather_lookup" || steps[3].FunctionResultStep.CallID != "c1" || !value(steps[3].FunctionResultStep.IsError) {
		t.Fatalf("history lost: %+v", steps)
	}
	parts := steps[3].FunctionResultStep.Result.ArrayOfFunctionResultSubcontent
	if len(parts) != 2 || parts[0].TextContent.Text != `{"error":"unavailable"}` || value(parts[1].ImageContent.Data) != base64.StdEncoding.EncodeToString([]byte("image")) {
		t.Fatalf("tool result lost: %+v", parts)
	}
	if value(steps[4].UserInputStep.Content[0].DocumentContent.Data) != base64.StdEncoding.EncodeToString([]byte("pdf")) {
		t.Fatal("PDF lost")
	}
}

func TestConvertInteraction_ThinkingAndToolChoice(t *testing.T) {
	for _, test := range []struct {
		name      string
		reasoning provider.ReasoningOptions
		level     interactions.ThinkingLevel
		summary   interactions.ThinkingSummaries
	}{
		{"default", provider.ReasoningOptions{}, "", interactions.ThinkingSummariesNone},
		{"minimal", provider.ReasoningOptions{Effort: provider.EffortMinimal}, interactions.ThinkingLevelLow, interactions.ThinkingSummariesNone},
		{"low", provider.ReasoningOptions{Effort: provider.EffortLow}, interactions.ThinkingLevelLow, interactions.ThinkingSummariesNone},
		{"medium", provider.ReasoningOptions{Effort: provider.EffortMedium}, interactions.ThinkingLevelMedium, interactions.ThinkingSummariesNone},
		{"high", provider.ReasoningOptions{Effort: provider.EffortHigh}, interactions.ThinkingLevelHigh, interactions.ThinkingSummariesNone},
		{"xhigh", provider.ReasoningOptions{Effort: provider.EffortXHigh}, interactions.ThinkingLevelHigh, interactions.ThinkingSummariesNone},
		{"maximum", provider.ReasoningOptions{Effort: provider.EffortMax, IncludeSummary: true}, interactions.ThinkingLevelHigh, interactions.ThinkingSummariesAuto},
		{"disabled", provider.ReasoningOptions{Type: provider.ReasoningTypeDisabled, Effort: provider.EffortHigh, IncludeSummary: true}, interactions.ThinkingLevelLow, interactions.ThinkingSummariesNone},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := convertInteraction("gemini-2.5-flash", nil, &provider.CompleteOptions{ReasoningOptions: &test.reasoning})
			if err != nil {
				t.Fatal(err)
			}
			if value(request.GenerationConfig.ThinkingLevel) != test.level || value(request.GenerationConfig.ThinkingSummaries) != test.summary {
				t.Fatalf("thinking not translated: %+v", request.GenerationConfig)
			}
		})
	}
	for _, choice := range []provider.ToolChoice{provider.ToolChoiceAuto, provider.ToolChoiceNone, provider.ToolChoiceAny} {
		request, err := convertInteraction("gemini-3.8-flash", nil, &provider.CompleteOptions{Tools: []provider.Tool{{Name: "lookup"}}, ToolOptions: &provider.ToolOptions{Choice: choice, Allowed: []string{"lookup"}}})
		if err != nil {
			t.Fatal(err)
		}
		selection := request.GenerationConfig.ToolChoice
		if choice == provider.ToolChoiceAny {
			allowed := selection.ToolChoiceConfig.AllowedTools
			if value(allowed.Mode) != interactions.ToolChoiceTypeAny || !reflect.DeepEqual(allowed.Tools, []string{"lookup"}) {
				t.Fatalf("allowed tools lost: %+v", allowed)
			}
		} else if value(selection.ToolChoiceType) != interactions.ToolChoiceType(choice) {
			t.Fatalf("choice lost: %+v", selection)
		}
	}
}

func TestConvertMessages_PreservesInterleavingAndLegacyIDs(t *testing.T) {
	messages := []provider.Message{{Role: provider.MessageRoleAssistant, Content: []provider.Content{
		provider.TextContent("first "), provider.ReasoningContent(provider.Reasoning{Summary: "think", Signature: "opaque_signature"}),
		provider.ToolCallContent(provider.ToolCall{ID: "c1::lookup::Ev8BAA==", Name: "lookup", Arguments: `{}`}), provider.TextContent(" last"),
	}}, provider.ToolMessage("c1::lookup::Ev8BAA==", "ok")}
	steps, err := convertMessages(messages)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 6 || steps[0].ModelOutputStep.Content[0].TextContent.Text != "first " || value(steps[1].ThoughtStep.Signature) != "opaque_signature" || value(steps[2].ThoughtStep.Signature) != "Ev8BAA==" || steps[3].FunctionCallStep.ID != "c1" || steps[4].ModelOutputStep.Content[0].TextContent.Text != " last" || steps[5].FunctionResultStep.CallID != "c1" {
		t.Fatalf("history order changed: %+v", steps)
	}
}

func TestConvertMessages_PreservesNumericArgumentsAndTextResults(t *testing.T) {
	arguments := `{"id":9007199254740993,"decimal":0.1234567890123456789}`
	steps, err := convertMessages([]provider.Message{
		{Role: provider.MessageRoleAssistant, Content: []provider.Content{provider.ToolCallContent(provider.ToolCall{ID: "c1", Name: "lookup", Arguments: arguments})}},
		{Role: provider.MessageRoleUser, Content: []provider.Content{provider.ToolResultContent(provider.ToolResult{ID: "c1", IsError: true, Parts: []provider.Part{{Text: "first"}, {Text: " second"}}})}},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(steps[0].FunctionCallStep.Arguments)
	if err != nil || string(encoded) != `{"decimal":0.1234567890123456789,"id":9007199254740993}` {
		t.Fatalf("tool argument precision lost: %s (%v)", encoded, err)
	}
	result := steps[1].FunctionResultStep
	if result.Result.Str == nil || *result.Result.Str != "first second" || !value(result.IsError) || value(result.Name) != "lookup" {
		t.Fatalf("plain-text result lost: %+v", result)
	}
}

func TestConvertMessages_RejectsInvalidContent(t *testing.T) {
	for _, messages := range [][]provider.Message{
		{{Role: provider.MessageRoleAssistant, Content: []provider.Content{provider.ToolCallContent(provider.ToolCall{ID: "c1", Name: "lookup", Arguments: `[1]`})}}},
		{{Role: provider.MessageRoleAssistant, Content: []provider.Content{provider.ToolCallContent(provider.ToolCall{ID: "c1", Name: "lookup", Arguments: `{} {}`})}}},
		{{Role: provider.MessageRoleUser, Content: []provider.Content{provider.FileContent(&provider.File{ContentType: "application/octet-stream", Content: []byte("binary")})}}},
		{{Role: provider.MessageRoleUser, Content: []provider.Content{provider.ToolResultContent(provider.ToolResult{ID: "c1", Parts: []provider.Part{{File: &provider.File{ContentType: "audio/wav", Content: []byte("audio")}}}})}}},
	} {
		if _, err := convertMessages(messages); err == nil {
			t.Fatal("expected explicit invalid request")
		}
	}
}
