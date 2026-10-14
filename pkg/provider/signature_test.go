package provider

import "testing"

func TestTagParseSignature(t *testing.T) {
	tagged := TagSignature("claude-sonnet-4-6", "abc:def")

	if tagged != "@claude-sonnet-4-6:abc:def" {
		t.Fatalf("tagged: %q", tagged)
	}

	realm, raw := ParseSignature(tagged)
	if realm != "claude-sonnet-4-6" || raw != "abc:def" {
		t.Errorf("parse: %q %q", realm, raw)
	}

	if realm, raw := ParseSignature("plain"); realm != "" || raw != "plain" {
		t.Errorf("untagged: %q %q", realm, raw)
	}

	if TagSignature("gpt-5.4", "") != "" {
		t.Error("empty signature must stay empty")
	}
}

func TestParseSignaturePreservesBinaryValues(t *testing.T) {
	for _, value := range []string{
		"@\x01:signature",
		"@\xff:signature",
		"@model:\x80\xff\x00",
		"@\x01:\x80\xff\x00",
	} {
		t.Run(value, func(t *testing.T) {
			if realm, raw := ParseSignature(value); realm != "" || raw != value {
				t.Fatalf("binary signature parsed as tagged: realm=%q raw=%q", realm, raw)
			}
		})
	}
}
