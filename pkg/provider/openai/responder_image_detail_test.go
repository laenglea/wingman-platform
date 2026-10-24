package openai

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestResponderRejectsInvalidImageDetail(t *testing.T) {
	responder, err := NewResponder("", "gpt-6-luna")
	if err != nil {
		t.Fatal(err)
	}
	file := &provider.File{Content: []byte("hi"), ContentType: "image/png", ImageDetail: provider.ImageDetail("invalid")}
	for _, content := range []provider.Content{
		provider.FileContent(file),
		provider.ToolResultContent(provider.ToolResult{ID: "f", Parts: []provider.Part{{File: file}}}),
		provider.ToolResultContent(provider.ToolResult{ID: "c", Kind: provider.ToolKindCustom, Parts: []provider.Part{{File: file}}}),
	} {
		input := []provider.Message{{Role: provider.MessageRoleUser, Content: []provider.Content{content}}}
		if _, err := responder.convertResponsesInput(input, false); err == nil {
			t.Fatal("accepted invalid image detail")
		}
	}
}
