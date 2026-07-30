package audio

// https://platform.openai.com/docs/api-reference/audio/createSpeech
type SpeechRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`

	Voice string   `json:"voice,omitempty"`
	Speed *float32 `json:"speed,omitempty"`

	Instructions string `json:"instructions,omitempty"`

	ResponseFormat string `json:"response_format,omitempty"`

	StreamFormat string `json:"stream_format,omitempty"`
}

type SpeechDeltaEvent struct {
	Type string `json:"type"`

	Audio string `json:"audio"`
}

type SpeechDoneEvent struct {
	Type string `json:"type"`
}

type TranscriptionLanguage struct {
	Code string `json:"code"`
}

type TranscriptionWord struct {
	Word string `json:"word"`

	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// https://platform.openai.com/docs/api-reference/audio/json-object
type Transcription struct {
	Task string `json:"task,omitempty"`

	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`

	Text string `json:"text"`

	Languages []TranscriptionLanguage `json:"languages,omitempty"`
}

// https://platform.openai.com/docs/api-reference/audio/verbose-json-object
type VerboseTranscription struct {
	Task string `json:"task"`

	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration"`

	Text string `json:"text"`

	Segments []VerboseTranscriptionSegment `json:"segments,omitempty"`
	Words    []TranscriptionWord           `json:"words,omitempty"`
}

type VerboseTranscriptionSegment struct {
	ID int `json:"id"`

	Start float64 `json:"start"`
	End   float64 `json:"end"`

	Text string `json:"text"`
}

// https://platform.openai.com/docs/api-reference/audio/diarized-json-object
type DiarizedTranscription struct {
	Task string `json:"task"`

	Duration float64 `json:"duration"`

	Text string `json:"text"`

	Segments []DiarizedTranscriptionSegment `json:"segments,omitempty"`
}

type DiarizedTranscriptionSegment struct {
	Type string `json:"type"`

	ID string `json:"id"`

	Speaker string `json:"speaker"`

	Start float64 `json:"start"`
	End   float64 `json:"end"`

	Text string `json:"text"`
}

type TranscriptionDeltaEvent struct {
	Type string `json:"type"`

	Delta string `json:"delta"`
}

type TranscriptionSegmentEvent struct {
	Type string `json:"type"`

	ID string `json:"id"`

	Speaker string `json:"speaker"`

	Start float64 `json:"start"`
	End   float64 `json:"end"`

	Text string `json:"text"`
}

type TranscriptionDoneEvent struct {
	Type string `json:"type"`

	Text string `json:"text"`

	Languages []TranscriptionLanguage `json:"languages,omitempty"`
}
