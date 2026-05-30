package exa

import "encoding/json"

// Research is served by /search with the deep-reasoning type.

type SearchRequest struct {
	Query string `json:"query"`

	Type string `json:"type,omitempty"`

	OutputSchema *OutputSchema `json:"outputSchema,omitempty"`
}

type OutputSchema struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type SearchResponse struct {
	Output *SearchOutput `json:"output,omitempty"`
}

type SearchOutput struct {
	// Content is a plain JSON string when outputSchema.type is "text".
	Content json.RawMessage `json:"content"`

	Grounding []Grounding `json:"grounding,omitempty"`
}

type Grounding struct {
	Field string `json:"field,omitempty"`

	Citations []Citation `json:"citations,omitempty"`

	Confidence string `json:"confidence,omitempty"`
}

type Citation struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}
