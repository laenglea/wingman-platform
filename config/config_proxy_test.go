package config

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyClientWithoutURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	client, err := (&proxyConfig{URL: ""}).proxyClient()
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}
