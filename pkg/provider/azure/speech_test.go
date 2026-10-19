package azure

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSpeechRequiresResourceURL(t *testing.T) {
	for _, endpoint := range []string{"", "eastus", "ftp://speech.test", "https://speech.test/tts", "https://speech.test/?key=secret", "https://speech.test/#fragment", "https://user:secret@speech.test", "https://%"} {
		t.Run(endpoint, func(t *testing.T) {
			if _, err := NewSynthesizer(endpoint, "tts"); err == nil {
				t.Fatal("synthesizer accepted an invalid resource URL")
			}
			if _, err := NewTranscriber(endpoint, "stt"); err == nil {
				t.Fatal("transcriber accepted an invalid resource URL")
			}
		})
	}
}

func TestSynthesizeResourceRequest(t *testing.T) {
	const input = `Grüezi & hello <world>! "Quotes" and 'apostrophes'. </voice><voice name="injected">`
	for _, tt := range []struct {
		name         string
		defaultVoice string
		options      *provider.SynthesizeOptions
		voice        string
		lang         string
		outputFormat string
		contentType  string
	}{
		{
			name: "default HD voice", voice: "en-US-Ava:DragonHDLatestNeural", lang: "en-US",
			outputFormat: "audio-24khz-160kbitrate-mono-mp3", contentType: "audio/mpeg",
		},
		{
			name: "configured voice", defaultVoice: "de-DE-Seraphina:DragonHDLatestNeural",
			options: &provider.SynthesizeOptions{Format: "wav"}, voice: "de-DE-Seraphina:DragonHDLatestNeural", lang: "de-DE",
			outputFormat: "riff-24khz-16bit-mono-pcm", contentType: "audio/wav",
		},
		{
			name: "request overrides configured voice", defaultVoice: "en-US-Ava:DragonHDLatestNeural",
			options: &provider.SynthesizeOptions{Voice: "fr-FR-Vivienne:DragonHDLatestNeural", Format: "pcm"},
			voice:   "fr-FR-Vivienne:DragonHDLatestNeural", lang: "fr-FR",
			outputFormat: "raw-24khz-16bit-mono-pcm", contentType: "audio/pcm",
		},
		{
			name: "OpenAI alias uses HD", options: &provider.SynthesizeOptions{Voice: "ALLOY", Format: "OPUS"},
			voice: "en-US-Alloy:DragonHDLatestNeural", lang: "en-US",
			outputFormat: "ogg-24khz-16bit-mono-opus", contentType: "audio/ogg",
		},
		{
			name: "Omni voice passes through", options: &provider.SynthesizeOptions{Voice: "de-DE-Conrad:DragonHDOmniLatestNeural"},
			voice: "de-DE-Conrad:DragonHDOmniLatestNeural", lang: "de-DE",
			outputFormat: "audio-24khz-160kbitrate-mono-mp3", contentType: "audio/mpeg",
		},
		{
			name: "voice attribute escaped", options: &provider.SynthesizeOptions{Voice: `en-US-Voice'"/><break/><voice name="other`},
			voice: `en-US-Voice'"/><break/><voice name="other`, lang: "en-US",
			outputFormat: "audio-24khz-160kbitrate-mono-mp3", contentType: "audio/mpeg",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Method != http.MethodPost || req.URL.String() != "https://speech.test/tts/cognitiveservices/v1" {
					t.Errorf("request = %s %s", req.Method, req.URL)
				}
				if req.Header.Get("Ocp-Apim-Subscription-Key") != "test-key" || req.Header.Get("Content-Type") != "application/ssml+xml" || req.Header.Get("User-Agent") == "" {
					t.Error("missing synthesis request headers")
				}
				if got := req.Header.Get("X-Microsoft-OutputFormat"); got != tt.outputFormat {
					t.Errorf("output format = %s, want %s", got, tt.outputFormat)
				}
				var document struct {
					XMLName xml.Name
					Lang    string `xml:"http://www.w3.org/XML/1998/namespace lang,attr"`
					Voices  []struct {
						Name string `xml:"name,attr"`
						Text string `xml:",chardata"`
					} `xml:"voice"`
				}
				if err := xml.NewDecoder(req.Body).Decode(&document); err != nil {
					t.Fatalf("invalid SSML: %v", err)
				}
				if document.XMLName.Space != "http://www.w3.org/2001/10/synthesis" || document.XMLName.Local != "speak" || document.Lang != tt.lang {
					t.Errorf("unexpected SSML root: %+v", document)
				}
				if len(document.Voices) != 1 || document.Voices[0].Name != tt.voice || document.Voices[0].Text != input {
					t.Errorf("SSML did not preserve voice and plain text: %+v", document.Voices)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("audio bytes"))}, nil
			})}
			synthesizer, err := NewSynthesizer("https://speech.test/", "azure-tts", WithToken("test-key"), WithVoice(tt.defaultVoice), WithClient(client))
			if err != nil {
				t.Fatal(err)
			}
			results := 0
			for result, err := range synthesizer.Synthesize(context.Background(), input, tt.options) {
				if err != nil {
					t.Fatal(err)
				}
				results++
				if result.ID == "" || result.Model != "azure-tts" || result.ContentType != tt.contentType || string(result.Content) != "audio bytes" {
					t.Errorf("unexpected synthesis: %+v", result)
				}
			}
			if calls != 1 || results != 1 {
				t.Fatalf("requests = %d, results = %d", calls, results)
			}
		})
	}
}

func TestSynthesizeRejectsUnsupportedFormatsBeforeRequest(t *testing.T) {
	for _, format := range []string{"aac", "flac", "unknown"} {
		t.Run(format, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("unsupported format sent upstream")
				return nil, nil
			})}
			synthesizer, err := NewSynthesizer("https://speech.test", "tts", WithClient(client))
			if err != nil {
				t.Fatal(err)
			}
			results := 0
			for result, err := range synthesizer.Synthesize(context.Background(), "Hello", &provider.SynthesizeOptions{Format: format}) {
				results++
				if result != nil || provider.CodeFromError(err, 0) != http.StatusBadRequest {
					t.Fatalf("result = %v, error = %v; want a validation error", result, err)
				}
			}
			if results != 1 {
				t.Fatalf("results = %d", results)
			}
		})
	}
}

func TestTranscribeResourceRequest(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.String() != "https://speech.test/speechtotext/transcriptions:transcribe?api-version=2025-10-15" {
			t.Errorf("request = %s %s", req.Method, req.URL)
		}
		if req.Header.Get("Ocp-Apim-Subscription-Key") != "test-key" {
			t.Error("missing resource key")
		}
		reader, err := req.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		parts := 0
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			parts++
			switch part.FormName() {
			case "audio":
				data, err := io.ReadAll(part)
				if err != nil || string(data) != "audio bytes" || part.FileName() != "hello.wav" || part.Header.Get("Content-Type") != "audio/wav" {
					t.Errorf("unexpected audio part: data=%q, error=%v", data, err)
				}
			case "definition":
				var definition struct {
					Locales     []string `json:"locales"`
					Diarization struct {
						Enabled     bool `json:"enabled"`
						MaxSpeakers int  `json:"maxSpeakers"`
					} `json:"diarization"`
				}
				if err := json.NewDecoder(part).Decode(&definition); err != nil {
					t.Fatal(err)
				}
				if len(definition.Locales) != 1 || definition.Locales[0] != "de-CH" || !definition.Diarization.Enabled || definition.Diarization.MaxSpeakers != 2 {
					t.Errorf("unexpected definition: %+v", definition)
				}
			default:
				t.Errorf("unexpected multipart field: %s", part.FormName())
			}
		}
		if parts != 2 {
			t.Errorf("multipart fields = %d", parts)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{
			"durationMilliseconds": 1500,
			"combinedPhrases": [{"text":"Grüezi!"}],
			"phrases": [{"text":"Grüezi!", "locale":"de-CH", "speaker":1, "offsetMilliseconds":100, "durationMilliseconds":900,
				"words":[{"text":"Grüezi!", "offsetMilliseconds":100, "durationMilliseconds":900}]}]
		}`))}, nil
	})}
	transcriber, err := NewTranscriber("https://speech.test/", "azure-stt", WithToken("test-key"), WithClient(client))
	if err != nil {
		t.Fatal(err)
	}
	results := 0
	for result, err := range transcriber.Transcribe(context.Background(), provider.File{Name: "hello.wav", ContentType: "audio/wav", Content: []byte("audio bytes")}, &provider.TranscribeOptions{Languages: []string{"de-CH"}, Diarize: true, Timestamps: true}) {
		if err != nil {
			t.Fatal(err)
		}
		results++
		if result.Text != "Grüezi!" || result.Language != "de-CH" || result.Duration != 1.5 || result.Model != "azure-stt" || result.ID == "" {
			t.Errorf("unexpected transcription: %+v", result)
		}
		if len(result.Segments) != 1 || result.Segments[0].Speaker != "1" || result.Segments[0].Start != 0.1 || result.Segments[0].End != 1 {
			t.Errorf("unexpected segments: %+v", result.Segments)
		}
		if len(result.Words) != 1 || result.Words[0].Word != "Grüezi!" || result.Words[0].Start != 0.1 || result.Words[0].End != 1 {
			t.Errorf("unexpected words: %+v", result.Words)
		}
	}
	if results != 1 {
		t.Fatalf("results = %d", results)
	}
}
