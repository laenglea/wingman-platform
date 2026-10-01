package google

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/adrianliechti/wingman/pkg/translator"

	"cloud.google.com/go/translate/apiv3/translatepb"
)

const maxDocumentSize = 20 * 1024 * 1024

var documentTypes = map[string]string{
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
}

func (c *Client) translateFile(ctx context.Context, input *translator.File, language string) (*translator.File, error) {
	if len(input.Content) == 0 {
		return nil, errors.New("google translator: no document content to translate")
	}

	if len(input.Content) > maxDocumentSize {
		return nil, errors.New("google translator: document exceeds the 20 MB limit")
	}

	contentType, err := documentContentType(input)

	if err != nil {
		return nil, err
	}

	client, parent, err := c.translationClient(ctx)

	if err != nil {
		return nil, err
	}

	// Google's default PDF mode supports scanned pages; leave native-only mode unset.
	result, err := client.TranslateDocument(ctx, &translatepb.TranslateDocumentRequest{
		Parent:             parent,
		TargetLanguageCode: language,
		DocumentInputConfig: &translatepb.DocumentInputConfig{
			MimeType: contentType,
			Source:   &translatepb.DocumentInputConfig_Content{Content: input.Content},
		},
		EnableRotationCorrection: contentType == "application/pdf",
	})

	if err != nil {
		return nil, err
	}

	document := result.GetDocumentTranslation()
	outputs := document.GetByteStreamOutputs()

	if len(outputs) != 1 || len(outputs[0]) == 0 {
		return nil, errors.New("google translator: expected one translated document")
	}

	if document.GetMimeType() != "" {
		contentType = document.GetMimeType()
	}

	return &translator.File{
		Name:        input.Name,
		Content:     outputs[0],
		ContentType: contentType,
	}, nil
}

func documentContentType(input *translator.File) (string, error) {
	contentType, _, _ := mime.ParseMediaType(input.ContentType)

	for _, supported := range documentTypes {
		if contentType == supported {
			return contentType, nil
		}
	}

	if contentType := documentTypes[strings.ToLower(filepath.Ext(input.Name))]; contentType != "" {
		return contentType, nil
	}

	if http.DetectContentType(input.Content) == "application/pdf" {
		return "application/pdf", nil
	}

	return "", fmt.Errorf("google translator: document format %q (%s): %w", input.Name, input.ContentType, translator.ErrUnsupported)
}
