package features_test

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adrianliechti/wingman/test/openai"
	"github.com/adrianliechti/wingman/test/openai/chat"
)

const audioSampleURL = "https://cdn.openai.com/API/docs/audio/alloy.wav"

func TestInputAudioRoundtrip(t *testing.T) {
	model := os.Getenv("TEST_OPENAI_AUDIO_MODEL")
	if model == "" {
		t.Skip("TEST_OPENAI_AUDIO_MODEL not set — skipping audio input test")
	}

	h := openai.New(t)

	audio := fetchAudio(t, audioSampleURL)
	encoded := base64.StdEncoding.EncodeToString(audio)

	body := map[string]any{
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "Transcribe the spoken audio. Respond with only the transcript."},
					{
						"type": "input_audio",
						"input_audio": map[string]any{
							"data":   encoded,
							"format": "wav",
						},
					},
				},
			},
		},
	}

	wingmanBody := chat.WithModel(body, model)

	ctx := context.Background()
	wingmanResp, err := h.Client.Post(ctx, h.Wingman, "/chat/completions", wingmanBody)
	if err != nil {
		t.Fatalf("wingman request failed: %v", err)
	}
	if wingmanResp.StatusCode != 200 {
		t.Fatalf("wingman returned status %d: %s", wingmanResp.StatusCode, string(wingmanResp.RawBody))
	}

	content := extractFirstContent(t, wingmanResp.Body)
	if content == "" {
		t.Fatalf("wingman returned empty content: %s", string(wingmanResp.RawBody))
	}

	t.Logf("transcript: %s", content)
}

func fetchAudio(t *testing.T, url string) []byte {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("create audio request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("fetch audio: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("audio fetch status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read audio: %v", err)
	}

	if len(data) < 1024 {
		t.Fatalf("audio sample too small (%d bytes)", len(data))
	}

	return data
}

func extractFirstContent(t *testing.T, body map[string]any) string {
	t.Helper()

	choices, ok := body["choices"].([]any)
	if !ok || len(choices) == 0 {
		return ""
	}

	choice, ok := choices[0].(map[string]any)
	if !ok {
		return ""
	}

	message, ok := choice["message"].(map[string]any)
	if !ok {
		return ""
	}

	if c, ok := message["content"].(string); ok {
		return strings.TrimSpace(c)
	}

	return ""
}
