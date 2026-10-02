package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adrianliechti/wingman/pkg/provider"

	"github.com/gorilla/websocket"
)

// newOpenAIWireCapture returns a session whose outgoing events are readable
// from the returned channel.
func newOpenAIWireCapture(t *testing.T) (*openAIRealtimeSession, <-chan map[string]any) {
	t.Helper()
	messages := make(chan map[string]any, 16)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(w, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var message map[string]any
			if err := conn.ReadJSON(&message); err != nil {
				return
			}
			messages <- message
		}
	}))
	t.Cleanup(server.Close)

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &openAIRealtimeSession{conn: conn}, messages
}

func readCaptured(t *testing.T, messages <-chan map[string]any) map[string]any {
	t.Helper()
	select {
	case message := <-messages:
		return message
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a captured event")
		return nil
	}
}

func TestOpenAIRealtimeConversationItemEditing(t *testing.T) {
	session, captured := newOpenAIWireCapture(t)
	ctx := t.Context()

	message := provider.Message{ID: "ctx_12345", Role: provider.MessageRoleSystem, Content: []provider.Content{provider.TextContent("context")}}
	if err := session.SendMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	created := readCaptured(t, captured)
	item, _ := created["item"].(map[string]any)
	if created["type"] != "conversation.item.create" || item["id"] != "ctx_12345" {
		t.Fatalf("create event = %#v, want the caller's item id", created)
	}

	if err := session.SendMessage(ctx, provider.Message{Role: provider.MessageRoleUser, Content: []provider.Content{provider.TextContent("hi")}}); err != nil {
		t.Fatal(err)
	}
	item, _ = readCaptured(t, captured)["item"].(map[string]any)
	if _, ok := item["id"]; ok {
		t.Fatalf("create event item = %#v, want no id when the caller supplied none", item)
	}

	if err := session.DeleteItem(ctx, "ctx_12345"); err != nil {
		t.Fatal(err)
	}
	deleted := readCaptured(t, captured)
	if deleted["type"] != "conversation.item.delete" || deleted["item_id"] != "ctx_12345" {
		t.Fatalf("delete event = %#v", deleted)
	}
	if err := session.DeleteItem(ctx, ""); err == nil {
		t.Fatal("DeleteItem accepted an empty id")
	}
}

func TestValidateOpenAIRealtimeAudioFormats(t *testing.T) {
	defaults := (&Realtime{}).Defaults()
	for _, encoding := range []provider.RealtimeAudioEncoding{
		provider.RealtimeAudioPCMU,
		provider.RealtimeAudioPCMA,
	} {
		t.Run(string(encoding), func(t *testing.T) {
			options := defaults
			format := provider.RealtimeAudioFormat{
				Encoding: encoding, SampleRate: 8000, SampleSize: 8, Channels: 1,
			}
			options.InputAudio = format
			options.OutputAudio = format
			if err := validateOpenAIRealtimeOptions(options); err != nil {
				t.Fatalf("valid G.711 format rejected: %v", err)
			}

			options.InputAudio.SampleSize = 16
			if err := validateOpenAIRealtimeOptions(options); err == nil {
				t.Fatal("16-bit G.711 format was accepted")
			}
		})
	}
}

func TestOpenAIRealtimeAudioFormatObject(t *testing.T) {
	pcm := openAIAudioFormat(provider.RealtimeAudioFormat{
		Encoding: provider.RealtimeAudioPCM, SampleRate: 24000, SampleSize: 16, Channels: 1,
	})
	if pcm["type"] != "audio/pcm" || pcm["rate"] != 24000 {
		t.Fatalf("PCM format = %#v", pcm)
	}

	g711 := openAIAudioFormat(provider.RealtimeAudioFormat{
		Encoding: provider.RealtimeAudioPCMU, SampleRate: 8000, SampleSize: 8, Channels: 1,
	})
	if g711["type"] != "audio/pcmu" {
		t.Fatalf("PCMU format = %#v", g711)
	}
	if _, ok := g711["rate"]; ok {
		t.Fatalf("PCMU format includes unsupported rate: %#v", g711)
	}
}

func TestOpenAIRealtimeSessionObjectForcesTracingOff(t *testing.T) {
	object := openAISessionObject((&Realtime{}).Defaults())
	tracing, ok := object["tracing"]
	if !ok {
		t.Fatal("session object does not set tracing")
	}
	if tracing != nil {
		t.Fatalf("session tracing = %#v, want null", tracing)
	}
}

func TestTranslateCurrentInputEvents(t *testing.T) {
	session := &openAIRealtimeSession{
		contents:    make(map[string]provider.RealtimeContentType),
		transcripts: make(map[string]bool),
	}

	timeout := session.translate([]byte(`{
		"type":"input_audio_buffer.timeout_triggered",
		"item_id":"item_timeout","audio_start_ms":13216,"audio_end_ms":19232
	}`))
	if len(timeout) != 1 || timeout[0].Type != provider.RealtimeEventInputTimeoutTriggered {
		t.Fatalf("timeout events = %#v", timeout)
	}
	if timeout[0].ItemID != "item_timeout" || timeout[0].AudioStart.Milliseconds() != 13216 || timeout[0].AudioEnd.Milliseconds() != 19232 {
		t.Fatalf("timeout event = %#v", timeout[0])
	}

	segment := session.translate([]byte(`{
		"type":"conversation.item.input_audio_transcription.segment",
		"item_id":"msg_011","content_index":2,"text":"hello","id":"seg_0001",
		"speaker":"spk_1","start":0.1,"end":0.4
	}`))
	if len(segment) != 1 || segment[0].Type != provider.RealtimeEventInputTranscriptionSegment {
		t.Fatalf("segment events = %#v", segment)
	}
	got := segment[0]
	if got.ItemID != "msg_011" || got.ContentIndex != 2 || got.Text != "hello" || got.SegmentID != "seg_0001" || got.Speaker != "spk_1" || got.SegmentStart != 0.1 || got.SegmentEnd != 0.4 {
		t.Fatalf("segment event = %#v", got)
	}
}
