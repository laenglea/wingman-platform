package azure

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"

	"github.com/google/uuid"
)

var _ provider.Synthesizer = (*Synthesizer)(nil)

type Synthesizer struct {
	*Config
}

func NewSynthesizer(endpoint, model string, options ...Option) (*Synthesizer, error) {
	cfg, err := newConfig(endpoint, model, options...)
	if err != nil {
		return nil, err
	}

	return &Synthesizer{
		Config: cfg,
	}, nil
}

func (s *Synthesizer) Synthesize(ctx context.Context, content string, options *provider.SynthesizeOptions) iter.Seq2[*provider.Synthesis, error] {
	return func(yield func(*provider.Synthesis, error) bool) {
		synthesis, err := s.synthesize(ctx, content, options)

		if err != nil {
			yield(nil, err)
			return
		}

		yield(synthesis, nil)
	}
}

func (s *Synthesizer) synthesize(ctx context.Context, content string, options *provider.SynthesizeOptions) (*provider.Synthesis, error) {
	if options == nil {
		options = new(provider.SynthesizeOptions)
	}

	voice := options.Voice
	if voice == "" {
		voice = s.voice
	}
	voice = mapVoice(voice)

	outputFormat, contentType, err := mapFormat(options.Format)
	if err != nil {
		return nil, err
	}

	ssml, err := buildSSML(voice, content)
	if err != nil {
		return nil, err
	}

	endpoint := s.url + "/tts/cognitiveservices/v1"

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(ssml))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", outputFormat)
	req.Header.Set("User-Agent", "wingman")

	if s.token != "" {
		req.Header.Set("Ocp-Apim-Subscription-Key", s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, convertError(resp)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &provider.Synthesis{
		ID:    uuid.NewString(),
		Model: s.model,

		Content:     data,
		ContentType: contentType,
	}, nil
}

// voiceMap maps OpenAI voice aliases to Azure Dragon HD personas.
var voiceMap = map[string]string{
	"alloy":   "en-US-Alloy:DragonHDLatestNeural",
	"ash":     "en-US-Andrew:DragonHDLatestNeural",
	"ballad":  "en-US-Brian:DragonHDLatestNeural",
	"coral":   "en-US-Emma:DragonHDLatestNeural",
	"echo":    "en-US-Andrew:DragonHDLatestNeural",
	"fable":   "en-US-Brian:DragonHDLatestNeural",
	"nova":    "en-US-Nova:DragonHDLatestNeural",
	"onyx":    "en-US-Steffan:DragonHDLatestNeural",
	"sage":    "en-US-Andrew:DragonHDLatestNeural",
	"shimmer": "en-US-Ava:DragonHDLatestNeural",
	"verse":   "en-US-Brian:DragonHDLatestNeural",
	"marin":   "en-US-Ava:DragonHDLatestNeural",
	"cedar":   "en-US-Andrew:DragonHDLatestNeural",
}

func mapVoice(voice string) string {
	if voice == "" {
		return defaultVoice
	}

	if mapped, ok := voiceMap[strings.ToLower(voice)]; ok {
		return mapped
	}

	// Already an Azure voice name — use as-is
	return voice
}

func buildSSML(voice, text string) (string, error) {
	// Extract lang from voice name (e.g. "en-US-JennyNeural" -> "en-US")
	lang := "en-US"
	parts := strings.SplitN(voice, "-", 3)
	if len(parts) >= 2 {
		lang = parts[0] + "-" + parts[1]
	}

	document := struct {
		XMLName xml.Name `xml:"speak"`
		Version string   `xml:"version,attr"`
		XMLNS   string   `xml:"xmlns,attr"`
		Lang    string   `xml:"xml:lang,attr"`
		Voice   struct {
			Name string `xml:"name,attr"`
			Text string `xml:",chardata"`
		} `xml:"voice"`
	}{
		Version: "1.0",
		XMLNS:   "http://www.w3.org/2001/10/synthesis",
		Lang:    lang,
	}
	document.Voice.Name = voice
	document.Voice.Text = text

	data, err := xml.Marshal(document)
	return string(data), err
}

// mapFormat keeps the Azure output encoding and returned MIME type consistent.
func mapFormat(format string) (string, string, error) {
	switch strings.ToLower(format) {
	case "", "mp3":
		return "audio-24khz-160kbitrate-mono-mp3", "audio/mpeg", nil
	case "opus":
		return "ogg-24khz-16bit-mono-opus", "audio/ogg", nil
	case "wav":
		return "riff-24khz-16bit-mono-pcm", "audio/wav", nil
	case "pcm":
		return "raw-24khz-16bit-mono-pcm", "audio/pcm", nil
	default:
		return "", "", provider.InvalidRequest(fmt.Errorf("azure speech does not support response_format %q; use mp3, opus, wav, or pcm", format))
	}
}
