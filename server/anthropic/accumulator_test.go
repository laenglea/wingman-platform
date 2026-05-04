package anthropic

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestStreamingAccumulatorEmitsCacheUsage(t *testing.T) {
	var events []StreamEvent
	acc := NewStreamingAccumulator("msg_123", "claude-test", func(event StreamEvent) error {
		events = append(events, event)
		return nil
	})

	err := acc.Add(provider.Completion{
		Usage: &provider.Usage{
			InputTokens:              0,
			CacheReadInputTokens:     40,
			CacheCreationInputTokens: 50,
		},
	})
	if err != nil {
		t.Fatalf("add completion: %v", err)
	}

	err = acc.Complete()
	if err != nil {
		t.Fatalf("complete stream: %v", err)
	}

	if len(events) < 2 {
		t.Fatalf("expected stream events, got %d", len(events))
	}

	start := events[0]
	if start.Type != StreamEventMessageStart {
		t.Fatalf("expected first event %q, got %q", StreamEventMessageStart, start.Type)
	}
	if start.Message == nil {
		t.Fatal("expected message_start payload")
	}
	if start.Message.Usage.CacheReadInputTokens != 40 {
		t.Fatalf("expected message_start cache read input tokens 40, got %d", start.Message.Usage.CacheReadInputTokens)
	}
	if start.Message.Usage.CacheCreationInputTokens != 50 {
		t.Fatalf("expected message_start cache creation input tokens 50, got %d", start.Message.Usage.CacheCreationInputTokens)
	}

	var delta *DeltaUsage
	for _, event := range events {
		if event.Type == StreamEventMessageDelta {
			delta = event.DeltaUsage
		}
	}
	if delta == nil {
		t.Fatal("expected message_delta usage")
	}
	if delta.CacheReadInputTokens != 40 {
		t.Fatalf("expected message_delta cache read input tokens 40, got %d", delta.CacheReadInputTokens)
	}
	if delta.CacheCreationInputTokens != 50 {
		t.Fatalf("expected message_delta cache creation input tokens 50, got %d", delta.CacheCreationInputTokens)
	}
}
