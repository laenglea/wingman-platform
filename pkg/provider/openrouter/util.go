package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func doRequest(ctx context.Context, client *http.Client, url, token string, body any, result any) error {
	data, err := json.Marshal(body)

	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))

	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return &provider.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

func extractMessage(result map[string]any) (map[string]any, error) {
	choices, ok := result["choices"].([]any)
	if !ok || len(choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	choice, ok := choices[0].(map[string]any)
	if !ok {
		return nil, errors.New("invalid choice in response")
	}

	message, ok := choice["message"].(map[string]any)
	if !ok {
		return nil, errors.New("no message in response")
	}

	return message, nil
}
