package audio

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestFormatSRT(t *testing.T) {
	segments := []provider.TranscriptionSegment{
		{Start: 0, End: 1.5, Text: " Hello "},
		{Start: 1.5, End: 3661.007, Text: "World"},
	}

	want := "1\n00:00:00,000 --> 00:00:01,500\nHello\n\n2\n00:00:01,500 --> 01:01:01,007\nWorld\n\n\n"

	if got := formatSRT(segments); got != want {
		t.Errorf("formatSRT() = %q, want %q", got, want)
	}

	if got := formatSRT(nil); got != "" {
		t.Errorf("formatSRT(nil) = %q, want empty", got)
	}
}

func TestFormatVTT(t *testing.T) {
	segments := []provider.TranscriptionSegment{
		{Start: 0, End: 1.5, Text: "Hello"},
	}

	want := "WEBVTT\n\n00:00:00.000 --> 00:00:01.500\nHello\n\n"

	if got := formatVTT(segments); got != want {
		t.Errorf("formatVTT() = %q, want %q", got, want)
	}
}

func TestSegmentsOfFallback(t *testing.T) {
	segments := segmentsOf(&provider.Transcription{Text: "Hello", Duration: 2})

	if len(segments) != 1 {
		t.Fatalf("segmentsOf() returned %d segments, want 1", len(segments))
	}

	if segments[0].Text != "Hello" || segments[0].End != 2 {
		t.Errorf("segmentsOf() = %+v", segments[0])
	}

	if got := segmentsOf(&provider.Transcription{Text: "   "}); got != nil {
		t.Errorf("segmentsOf() = %+v, want nil", got)
	}
}

func TestDecodeSpeakerReference(t *testing.T) {
	reference, err := decodeSpeakerReference("data:audio/wav;base64,aGVsbG8=")

	if err != nil {
		t.Fatalf("decodeSpeakerReference() error = %v", err)
	}

	if string(reference.Content) != "hello" {
		t.Errorf("content = %q, want %q", reference.Content, "hello")
	}

	if reference.ContentType != "audio/wav" {
		t.Errorf("content type = %q, want %q", reference.ContentType, "audio/wav")
	}

	if _, err := decodeSpeakerReference("https://example.com/sample.wav"); err == nil {
		t.Error("decodeSpeakerReference() expected error for non data URL")
	}
}

func TestDiarizedTranscription(t *testing.T) {
	result := diarizedTranscription(&provider.Transcription{
		Text:     "Hi there",
		Duration: 4,

		Segments: []provider.TranscriptionSegment{
			{ID: "seg_0", Speaker: "A", Start: 0, End: 2, Text: "Hi"},
			{ID: "seg_1", Speaker: "B", Start: 2, End: 4, Text: "there"},
		},
	})

	if result.Task != "transcribe" || result.Duration != 4 {
		t.Errorf("diarizedTranscription() = %+v", result)
	}

	if len(result.Segments) != 2 {
		t.Fatalf("got %d segments, want 2", len(result.Segments))
	}

	if result.Segments[1].Speaker != "B" || result.Segments[1].Type != "transcript.text.segment" {
		t.Errorf("segment = %+v", result.Segments[1])
	}
}

func TestWriteTranscriptionStream(t *testing.T) {
	deltas := []*provider.Transcription{
		{Text: "Hi"},
		{Segments: []provider.TranscriptionSegment{{ID: "seg_0", Speaker: "A", Start: 0, End: 1, Text: "Hi"}}},
		{Text: " there"},
		{Languages: []string{"en"}},
	}

	seq := func(yield func(*provider.Transcription, error) bool) {
		for _, delta := range deltas {
			if !yield(delta, nil) {
				return
			}
		}
	}

	w := httptest.NewRecorder()
	writeTranscriptionStream(w, seq)

	if got := w.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("content type = %q, want %q", got, "text/event-stream")
	}

	body := w.Body.String()

	for _, want := range []string{
		`"type":"transcript.text.delta","delta":"Hi"`,
		`"type":"transcript.text.segment"`,
		`"speaker":"A"`,
		`"type":"transcript.text.done","text":"Hi there"`,
		`"languages":[{"code":"en"}]`,
		"data: [DONE]",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n%s", want, body)
		}
	}
}

func TestWriteTranscriptionStreamError(t *testing.T) {
	seq := func(yield func(*provider.Transcription, error) bool) {
		yield(nil, errors.New("boom"))
	}

	w := httptest.NewRecorder()
	writeTranscriptionStream(w, seq)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}

	if !strings.Contains(w.Body.String(), "boom") {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestDurationOfFallsBackToSegments(t *testing.T) {
	transcription := &provider.Transcription{
		Segments: []provider.TranscriptionSegment{{End: 1}, {End: 4.5}},
	}

	if got := durationOf(transcription); got != 4.5 {
		t.Errorf("durationOf() = %v, want 4.5", got)
	}

	transcription.Duration = 9

	if got := durationOf(transcription); got != 9 {
		t.Errorf("durationOf() = %v, want 9", got)
	}
}
