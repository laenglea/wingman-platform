package provider

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Reasoning and compaction signatures (Anthropic thinking signatures, OpenAI
// encrypted reasoning, Gemini thought signatures) are opaque blobs that only
// the backend which produced them can verify — and "backend" is as narrow as
// one model behind one set of credentials. Wire formats give them no origin,
// so signatures are tagged with the realm that produced them on the way out
// and only replayed into the same realm (see adapter/signatures).
//
// Tagged form: "@<realm>:<signature>". Untagged values predate tagging and
// are treated as the replaying realm's own.
const signatureTagPrefix = "@"

// TagSignature prefixes a raw signature with its realm. Empty signatures
// stay empty.
func TagSignature(realm, signature string) string {
	if signature == "" || realm == "" {
		return signature
	}

	return signatureTagPrefix + realm + ":" + signature
}

// ParseSignature splits a tagged signature into realm and raw value. An
// untagged value comes back with an empty realm.
func ParseSignature(value string) (realm, signature string) {
	raw, ok := strings.CutPrefix(value, signatureTagPrefix)
	if !ok {
		return "", value
	}

	realm, signature, ok = strings.Cut(raw, ":")
	if !ok || !validSignatureRealm(realm) || !validSignaturePayload(signature) {
		return "", value
	}

	return realm, signature
}

func validSignatureRealm(realm string) bool {
	if realm == "" || !utf8.ValidString(realm) {
		return false
	}

	// Model aliases may contain spaces and Unicode; binary signatures must
	// still be distinguished from realm tags.
	for _, r := range realm {
		if !unicode.IsPrint(r) {
			return false
		}
	}

	return true
}

func validSignaturePayload(signature string) bool {
	if signature == "" {
		return false
	}

	for i := 0; i < len(signature); i++ {
		if signature[i] < ' ' || signature[i] > '~' {
			return false
		}
	}

	return true
}
