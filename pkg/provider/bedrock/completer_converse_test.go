package bedrock

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// TestConvertConverseInput_EffortStaysInAdditionalFields pins the effort to
// additionalModelRequestFields: Claude 4.6 on Bedrock rejects the typed
// outputConfig.effort field but accepts output_config.effort there.
func TestConvertConverseInput_EffortStaysInAdditionalFields(t *testing.T) {
	c := &Completer{Config: &Config{model: "eu.anthropic.claude-opus-4-6-v1"}}

	req, err := c.convertConverseInput([]provider.Message{provider.UserMessage("hi")}, &provider.CompleteOptions{
		ReasoningOptions: &provider.ReasoningOptions{Type: provider.ReasoningTypeAdaptive, Effort: provider.EffortLow},
	})
	if err != nil {
		t.Fatal(err)
	}

	if req.OutputConfig != nil {
		t.Fatalf("effort must not use the typed field: %+v", req.OutputConfig)
	}
	if req.AdditionalModelRequestFields == nil {
		t.Fatal("expected additional fields")
	}
	data, err := req.AdditionalModelRequestFields.MarshalSmithyDocument()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"output_config":{"effort":"low"}`) || !strings.Contains(string(data), "adaptive") {
		t.Fatalf("additional fields = %s", data)
	}
}

// TestConvertConverseInput_NativeOutputFormat verifies models with structured
// outputs take the schema as outputConfig.textFormat: the schema is reduced to
// the grammar subset, closed with additionalProperties: false, and no forced
// tool is added.
func TestConvertConverseInput_NativeOutputFormat(t *testing.T) {
	c := &Completer{Config: &Config{model: "anthropic.claude-sonnet-4-6"}}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string", "minLength": float64(1)},
		},
		"required": []string{"title"},
	}

	req, err := c.convertConverseInput([]provider.Message{provider.UserMessage("Return JSON.")}, &provider.CompleteOptions{
		Schema: &provider.Schema{Name: "classify_chat", Description: "Classify the chat", Properties: schema},
	})
	if err != nil {
		t.Fatal(err)
	}

	if req.ToolConfig != nil {
		t.Fatalf("schema must not become a forced tool: %+v", req.ToolConfig)
	}
	if req.OutputConfig == nil || req.OutputConfig.TextFormat == nil {
		t.Fatalf("missing text format: %+v", req.OutputConfig)
	}

	format := req.OutputConfig.TextFormat
	if format.Type != types.OutputFormatTypeJsonSchema {
		t.Fatalf("format type = %q", format.Type)
	}
	structure, ok := format.Structure.(*types.OutputFormatStructureMemberJsonSchema)
	if !ok {
		t.Fatalf("structure = %T", format.Structure)
	}
	if aws.ToString(structure.Value.Name) != "classify_chat" || aws.ToString(structure.Value.Description) != "Classify the chat" {
		t.Fatalf("definition = %+v", structure.Value)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(aws.ToString(structure.Value.Schema)), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["additionalProperties"] != false {
		t.Fatalf("schema not closed: %v", decoded)
	}
	title := decoded["properties"].(map[string]any)["title"].(map[string]any)
	if _, ok := title["minLength"]; ok {
		t.Fatalf("unsupported constraint reached the grammar: %v", title)
	}
	if _, ok := schema["additionalProperties"]; ok {
		t.Fatal("input schema mutated")
	}
}

// TestConvertConverseInput_SchemaWithoutPropertiesForcesTool verifies JSON
// mode without a schema keeps the forced-tool emulation even on models with
// native output formats.
func TestConvertConverseInput_SchemaWithoutPropertiesForcesTool(t *testing.T) {
	c := &Completer{Config: &Config{model: "anthropic.claude-sonnet-4-6"}}

	req, err := c.convertConverseInput([]provider.Message{provider.UserMessage("Return JSON.")}, &provider.CompleteOptions{
		Schema: &provider.Schema{Name: "json"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if req.OutputConfig != nil {
		t.Fatalf("unexpected output config: %+v", req.OutputConfig)
	}
	if req.ToolConfig == nil || len(req.ToolConfig.Tools) == 0 {
		t.Fatal("expected forced schema tool")
	}
}

// TestConverseAdditionalFields_NativeSchemaKeepsThinking verifies native
// output formats do not force thinking off the way a forced tool call does.
func TestConverseAdditionalFields_NativeSchemaKeepsThinking(t *testing.T) {
	c := &Completer{Config: &Config{model: "anthropic.claude-sonnet-4-6"}}

	fields, thinking := c.converseAdditionalFields(nil, &provider.CompleteOptions{
		Schema:           &provider.Schema{Name: "classify", Properties: testSchema},
		ReasoningOptions: &provider.ReasoningOptions{Type: provider.ReasoningTypeAdaptive},
	})

	if !thinking.Enabled {
		t.Fatal("expected thinking enabled with native output format")
	}
	got, _ := fields["thinking"].(map[string]any)
	if got["type"] != "adaptive" {
		t.Fatalf("thinking: got %v, want adaptive", fields["thinking"])
	}
}

func systemTexts(blocks []types.SystemContentBlock) []string {
	var texts []string
	for _, block := range blocks {
		if text, ok := block.(*types.SystemContentBlockMemberText); ok {
			texts = append(texts, text.Value)
		}
	}
	return texts
}

func messageRoles(messages []types.Message) []types.ConversationRole {
	roles := make([]types.ConversationRole, 0, len(messages))
	for _, message := range messages {
		roles = append(roles, message.Role)
	}
	return roles
}

// Converse requires system instructions in the top-level field, even for
// models that accept mid-conversation system messages through InvokeModel.
func TestConvertConverseInput_MidConversationSystem(t *testing.T) {
	history := []provider.Message{
		provider.SystemMessage("Be brief."),
		provider.UserMessage("Start"),
		{Role: provider.MessageRoleSystem, Content: []provider.Content{
			provider.InstructionsContent(provider.Instructions{Text: "Answer in German.", Scope: provider.InstructionScopeConversation}),
			provider.InstructionsContent(provider.Instructions{Text: "Expired instruction.", Scope: provider.InstructionScopeTurn}),
		}},
		provider.AssistantMessage("Erste Antwort"),
		provider.UserMessage("Continue"),
		{Role: provider.MessageRoleSystem, Content: []provider.Content{
			provider.InstructionsContent(provider.Instructions{Text: "Keep this answer short.", Scope: provider.InstructionScopeTurn}),
		}},
	}

	for _, model := range []string{"eu.anthropic.claude-opus-5", "eu.anthropic.claude-opus-4-8", "eu.anthropic.claude-fable-5", "anthropic.claude-sonnet-4-6"} {
		t.Run(model, func(t *testing.T) {
			c := &Completer{Config: &Config{model: model}}
			req, err := c.convertConverseInput(history, &provider.CompleteOptions{})
			if err != nil {
				t.Fatal(err)
			}
			wantRoles := []types.ConversationRole{types.ConversationRoleUser, types.ConversationRoleAssistant, types.ConversationRoleUser}
			if got := messageRoles(req.Messages); !slices.Equal(got, wantRoles) {
				t.Fatalf("roles = %v, want %v", got, wantRoles)
			}
			wantSystem := []string{"Be brief.", "Answer in German.", "Keep this answer short."}
			if got := systemTexts(req.System); !slices.Equal(got, wantSystem) {
				t.Fatalf("system = %v, want %v", got, wantSystem)
			}
		})
	}
}

func TestConvertFormats_AddedTypes(t *testing.T) {
	for mime, want := range map[string]types.DocumentFormat{
		"application/msword":       types.DocumentFormatDoc,
		"application/vnd.ms-excel": types.DocumentFormatXls,
		"text/html":                types.DocumentFormatHtml,
	} {
		if got, ok := convertDocumentFormat(mime); !ok || got != want {
			t.Errorf("%s = %q, want %q", mime, got, want)
		}
	}

	for mime, want := range map[string]types.VideoFormat{
		"video/x-flv":    types.VideoFormatFlv,
		"video/mpeg":     types.VideoFormatMpeg,
		"video/x-ms-wmv": types.VideoFormatWmv,
		"video/3gpp":     types.VideoFormatThreeGp,
	} {
		if got, ok := convertVideoFormat(mime); !ok || got != want {
			t.Errorf("%s = %q, want %q", mime, got, want)
		}
	}
}

func TestConvertToolResultFile_Video(t *testing.T) {
	block, err := convertToolResultFile(&provider.File{ContentType: "video/mp4", Content: []byte{1, 2, 3}})
	if err != nil {
		t.Fatal(err)
	}

	video, ok := block.(*types.ToolResultContentBlockMemberVideo)
	if !ok || video.Value.Format != types.VideoFormatMp4 {
		t.Fatalf("block = %T %+v", block, block)
	}
	if source, ok := video.Value.Source.(*types.VideoSourceMemberBytes); !ok || len(source.Value) != 3 {
		t.Fatalf("source = %T", video.Value.Source)
	}
}
