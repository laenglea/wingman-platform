package exa

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/adrianliechti/wingman/server/web"
	"github.com/google/uuid"
)

func (c ContentOptions) validate() (bool, int, error) {
	for _, field := range []struct {
		name  string
		value json.RawMessage
	}{{"highlights", c.Highlights}, {"summary", c.Summary}} {
		if len(field.value) != 0 && string(field.value) != "false" && string(field.value) != "null" {
			return false, 0, fmt.Errorf("%s is unsupported", field.name)
		}
	}
	if len(c.Text) == 0 {
		return false, 0, nil
	}
	if string(c.Text) == "false" || string(c.Text) == "null" {
		return false, 0, nil
	}
	if string(c.Text) == "true" {
		return true, 0, nil
	}
	var limits struct {
		MaxCharacters *int `json:"maxCharacters"`
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(c.Text, &fields); err != nil || fields == nil {
		return false, 0, errors.New("text must be a boolean or an object with maxCharacters")
	}
	for name := range fields {
		if name != "maxCharacters" {
			return false, 0, fmt.Errorf("unsupported text option %q", name)
		}
	}
	if err := json.Unmarshal(c.Text, &limits); err != nil {
		return false, 0, errors.New("maxCharacters must be an integer")
	}
	if limits.MaxCharacters == nil {
		return true, 0, nil
	}
	if *limits.MaxCharacters < 1 || *limits.MaxCharacters > 1000000 {
		return false, 0, errors.New("maxCharacters must be between 1 and 1000000")
	}
	return true, *limits.MaxCharacters, nil
}

func (h *Handler) handleContents(w http.ResponseWriter, r *http.Request) {
	var request ContentsRequest
	if err := web.ReadRequest(w, r, &request, nil); err != nil {
		web.WriteError(w, r, 400, err.Error())
		return
	}
	model, err := request.Provider(r)
	if err != nil {
		web.WriteError(w, r, 400, err.Error())
		return
	}
	if len(request.URLs) != 0 && len(request.IDs) != 0 {
		web.WriteError(w, r, 400, "use urls or ids, not both")
		return
	}
	if len(request.URLs) == 0 {
		request.URLs = request.IDs
	}
	if err := web.ValidateURLs(request.URLs); err != nil {
		web.WriteError(w, r, 400, err.Error())
		return
	}
	// This extraction subset defaults to text; search content options remain
	// opt-in, including when an empty contents object is supplied.
	if len(request.Text) == 0 {
		request.Text = json.RawMessage("true")
	}
	text, limit, err := request.ContentOptions.validate()
	if err != nil {
		web.WriteError(w, r, 400, err.Error())
		return
	}
	if !text {
		web.WriteError(w, r, 400, "contents requires text retrieval")
		return
	}
	if limit == 0 {
		limit = 10000
	}
	p, err := h.Scraper(r, model)
	if err != nil {
		web.WriteError(w, r, web.StatusOf(err), err.Error())
		return
	}
	response := Response{RequestID: uuid.NewString(), Results: []Result{}, Statuses: []Status{}}
	for _, result := range web.Extract(r.Context(), p, request.URLs, 0) {
		response.Statuses = append(response.Statuses, status(result))
		if result.Err == nil {
			text := web.Truncate(result.Text, limit)
			response.Results = append(response.Results, Result{ID: result.URL, URL: result.URL, Text: &text})
		}
	}
	web.WriteJSON(w, response)
}

func status(e web.Extraction) Status {
	status := Status{ID: e.URL, Status: "success"}
	if e.Err != nil {
		status.Status = "error"
		status.Error = &struct {
			Tag string `json:"tag"`
		}{Tag: "FETCH_ERROR"}
	}
	return status
}
