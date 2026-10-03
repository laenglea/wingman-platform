package google

import (
	"crypto/rand"
	"encoding/base64"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider/internal/toolid"
)

func generateCallID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func StripToolIDSignature(s string) string {
	return toolid.StripSignature(s)
}

// EncodeThoughtSignature keeps arbitrary signature bytes safe through the
// JSON strings used by Wingman's public endpoints.
func EncodeThoughtSignature(signature []byte) string {
	if len(signature) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(signature)
}

// DecodeThoughtSignature also accepts raw signatures from older histories.
func DecodeThoughtSignature(signature string) []byte {
	if signature == "" {
		return nil
	}
	if data, err := base64.StdEncoding.DecodeString(signature); err == nil {
		return data
	}
	return []byte(signature)
}

// parseToolID reads older id::name::base64sig histories. New Interactions
// calls use the upstream ID directly and carry signatures on thought steps.
func parseToolID(s string) (id, name, signature string) {
	parts := strings.SplitN(s, "::", 3)
	id = parts[0]
	if len(parts) > 1 {
		name = parts[1]
	}
	if len(parts) > 2 {
		signature = parts[2]
	}
	return
}
