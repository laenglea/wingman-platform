package image

import (
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/adrianliechti/wingman/pkg/policy"
	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/server/openai/shared"
)

func (h *Handler) handleImageEdit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	model := r.FormValue("model")
	prompt := r.FormValue("prompt")

	if err := h.Policy.Verify(r.Context(), policy.ResourceModel, model, policy.ActionAccess); err != nil {
		writeError(w, http.StatusNotFound, shared.ModelNotFound(model))
		return
	}

	renderer, err := h.Renderer(model)

	if err != nil {
		writeError(w, http.StatusNotFound, shared.ModelNotFound(model))
		return
	}

	var images []provider.File

	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		for _, field := range []string{"image", "image[]"} {
			for _, header := range r.MultipartForm.File[field] {
				file, err := header.Open()
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}

				data, err := io.ReadAll(file)
				file.Close()

				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}

				contentType := header.Header.Get("Content-Type")
				if mediatype, _, err := mime.ParseMediaType(contentType); err == nil {
					contentType = mediatype
				}

				images = append(images, provider.File{
					Name:        header.Filename,
					Content:     data,
					ContentType: contentType,
				})
			}
		}
	}

	if len(images) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("no image provided"))
		return
	}

	options := &provider.RenderOptions{
		Images: images,

		Aspect:     parseAspect(r.FormValue("size"), r.FormValue("aspect_ratio")),
		Quality:    parseQuality(r.FormValue("quality")),
		Resolution: provider.ParseResolution(r.FormValue("resolution")),

		Background: provider.ParseBackground(r.FormValue("background")),
		Format:     provider.ParseFormat(r.FormValue("output_format")),
	}

	image, err := renderer.Render(r.Context(), prompt, options)

	if err != nil {
		writeError(w, provider.CodeFromError(err, http.StatusBadRequest), err)
		return
	}

	result := ImageList{}

	if r.FormValue("response_format") == "url" {
		result.Images = []Image{
			{
				URL: "data:" + image.ContentType + ";base64," + base64.StdEncoding.EncodeToString(image.Content),
			},
		}
	} else {
		result.Images = []Image{
			{
				B64JSON: base64.StdEncoding.EncodeToString(image.Content),
			},
		}
	}

	writeJson(w, result)
}
