package provider

import (
	"net"
	"net/http"
	"time"
)

// DefaultClient is used by providers when no custom HTTP client is configured.
// Unlike http.DefaultClient it bounds connection setup and time-to-first-byte,
// so a hung upstream cannot stall a request indefinitely. It deliberately sets
// no overall timeout: streaming completions may legitimately run for many minutes.
// Its transport is wrapped with instrumentation by otel.Setup.
var DefaultClient = &http.Client{
	Transport: DefaultTransport(),
}

// DefaultTransport returns a transport with sane timeouts and pooling defaults
// for long-running LLM requests.
func DefaultTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,

		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,

		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 5 * time.Minute,
		ExpectContinueTimeout: 1 * time.Second,

		// No global idle cap: the pool is shared across all provider hosts
		// and is already bounded by IdleConnTimeout
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 32,
		IdleConnTimeout:     90 * time.Second,
	}
}
