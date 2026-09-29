package harness

// Endpoint represents an API target.
type Endpoint struct {
	Name    string
	BaseURL string // e.g. "http://localhost:4242/v1"
	APIKey  string
}
