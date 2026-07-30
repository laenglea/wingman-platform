package api

import (
	"io"
	"mime"
	"net/http"

	"github.com/adrianliechti/wingman/pkg/policy"
	"github.com/adrianliechti/wingman/pkg/provider"
)

func (h *Handler) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	model := valueModel(r)

	language := valueLanguage(r)
	instructions := valueInput(r)

	p, err := h.Transcriber(model)

	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := h.Policy.Verify(r.Context(), policy.ResourceModel, model, policy.ActionAccess); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	file, header, err := r.FormFile("file")

	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	defer file.Close()

	data, err := io.ReadAll(file)

	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	contentType := header.Header.Get("Content-Type")

	if mediatype, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediatype
	}

	input := provider.File{
		Name: header.Filename,

		Content:     data,
		ContentType: contentType,
	}

	options := &provider.TranscribeOptions{
		Instructions: instructions,

		Keywords: r.Form["keywords"],
	}

	if language != "" {
		options.Languages = []string{language}
	}

	if languages := r.Form["languages"]; len(languages) > 0 {
		options.Languages = languages
	}

	var acc provider.TranscriptionAccumulator

	for delta, err := range p.Transcribe(r.Context(), input, options) {
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		acc.Add(*delta)
	}

	transcription := acc.Result()

	w.Header().Set("Content-Type", "text/plain")

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, transcription.Text)
}
