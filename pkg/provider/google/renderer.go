package google

import (
	"context"
	"errors"

	"github.com/adrianliechti/wingman/pkg/provider"
	"google.golang.org/genai"
)

var _ provider.Renderer = (*Renderer)(nil)

type Renderer struct {
	*Config
}

func NewRenderer(model string, options ...Option) (*Renderer, error) {
	cfg := &Config{
		model: model,
	}

	for _, option := range options {
		option(cfg)
	}

	return &Renderer{
		Config: cfg,
	}, nil
}

func (r *Renderer) Render(ctx context.Context, input string, options *provider.RenderOptions) (*provider.Rendering, error) {
	if options == nil {
		options = new(provider.RenderOptions)
	}

	client, err := r.newClient(ctx)

	if err != nil {
		return nil, err
	}

	parts := []*genai.Part{
		genai.NewPartFromText(input),
	}

	for _, i := range options.Images {
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: i.ContentType,
				Data:     i.Content,
			},
		})
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	image, err := client.Models.GenerateContent(ctx, r.model, contents, imageConfig(options))

	if err != nil {
		return nil, convertError(err)
	}

	for _, candidate := range image.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			if part.InlineData == nil {
				continue
			}

			return &provider.Rendering{
				ID:    image.ResponseID,
				Model: r.model,

				Content:     part.InlineData.Data,
				ContentType: part.InlineData.MIMEType,
			}, nil
		}
	}

	// A blocked prompt or a refused candidate carries no image.
	reason := "no image in response"

	if feedback := image.PromptFeedback; feedback != nil && feedback.BlockReason != "" {
		reason = "prompt blocked: " + string(feedback.BlockReason)
	} else if len(image.Candidates) > 0 && image.Candidates[0].FinishReason != "" {
		reason = "no image in response: " + string(image.Candidates[0].FinishReason)
	}

	return nil, errors.New("gemini: " + reason)
}

var googleAspects = []provider.AspectRatio{
	provider.AspectRatio1x1,
	provider.AspectRatio2x3,
	provider.AspectRatio3x2,
	provider.AspectRatio3x4,
	provider.AspectRatio4x3,
	provider.AspectRatio4x5,
	provider.AspectRatio5x4,
	provider.AspectRatio9x16,
	provider.AspectRatio16x9,
	provider.AspectRatio21x9,
}

func imageConfig(options *provider.RenderOptions) *genai.GenerateContentConfig {
	config := &genai.ImageConfig{}

	if options.Aspect != "" {
		config.AspectRatio = string(options.Aspect.Nearest(googleAspects))
	}

	switch options.Resolution {
	case provider.Resolution512:
		config.ImageSize = "512"
	case provider.Resolution1K:
		config.ImageSize = "1K"
	case provider.Resolution2K:
		config.ImageSize = "2K"
	case provider.Resolution4K:
		config.ImageSize = "4K"
	}

	if config.AspectRatio == "" && config.ImageSize == "" {
		return nil
	}

	return &genai.GenerateContentConfig{
		ImageConfig: config,
	}
}
