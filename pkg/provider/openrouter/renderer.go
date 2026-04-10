package openrouter

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/google/uuid"
)

var _ provider.Renderer = (*Renderer)(nil)

var dataURLPattern = regexp.MustCompile(`data:[a-zA-Z]+/[a-zA-Z0-9.+_-]+;base64,\s*(.+)`)

type Renderer struct {
	*Config
}

func NewRenderer(model string, options ...Option) (*Renderer, error) {
	return &Renderer{
		Config: newConfig(model, options...),
	}, nil
}

func (r *Renderer) Render(ctx context.Context, input string, options *provider.RenderOptions) (*provider.Rendering, error) {
	if options == nil {
		options = new(provider.RenderOptions)
	}

	var content any

	if len(options.Images) == 0 {
		content = input
	} else {
		parts := []map[string]any{
			{
				"type": "text",
				"text": input,
			},
		}

		for _, img := range options.Images {
			contentType := img.ContentType
			if contentType == "" {
				contentType = "image/png"
			}

			dataURL := fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(img.Content))

			parts = append(parts, map[string]any{
				"type": "image_url",
				"image_url": map[string]any{
					"url": dataURL,
				},
			})
		}

		content = parts
	}

	body := map[string]any{
		"model": r.model,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
		"modalities": []string{"image", "text"},
		"stream":     false,
	}

	var result map[string]any

	if err := doRequest(ctx, r.client, r.url+"/chat/completions", r.token, body, &result); err != nil {
		return nil, err
	}

	message, err := extractMessage(result)

	if err != nil {
		return nil, err
	}

	images, ok := message["images"].([]any)

	if !ok || len(images) == 0 {
		return nil, errors.New("no images in response")
	}

	img, ok := images[0].(map[string]any)

	if !ok {
		return nil, errors.New("invalid image in response")
	}

	imageURL, ok := img["image_url"].(map[string]any)

	if !ok {
		return nil, errors.New("no image URL in response")
	}

	url, ok := imageURL["url"].(string)

	if !ok || url == "" {
		return nil, errors.New("no image URL in response")
	}

	if !strings.HasPrefix(url, "data:") {
		return nil, errors.New("unsupported image URL format: expected data URL")
	}

	match := dataURLPattern.FindStringSubmatch(url)

	if len(match) != 2 {
		return nil, errors.New("invalid data URL")
	}

	data, err := base64.StdEncoding.DecodeString(match[1])

	if err != nil {
		return nil, err
	}

	return &provider.Rendering{
		ID:    uuid.NewString(),
		Model: r.model,

		Content:     data,
		ContentType: "image/png",
	}, nil
}
