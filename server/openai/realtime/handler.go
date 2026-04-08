package realtime

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type Handler struct {
	baseURL string
	apiKey  string
}

func New() *Handler {
	apiKey := os.Getenv("REALTIME_API_KEY")
	baseURL := os.Getenv("REALTIME_BASE_URL")

	if baseURL == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")

		if apiKey == "" {
			return nil
		}

		baseURL = os.Getenv("OPENAI_BASE_URL")

		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
	}

	return &Handler{
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

func (h *Handler) Attach(r chi.Router) {
	r.HandleFunc("/realtime", h.handleRealtime)
}

func (h *Handler) dial(r *http.Request) (*websocket.Conn, *http.Response, error) {
	u, _ := url.Parse(h.baseURL)

	u.Scheme = "wss"
	u.Path = "/v1/realtime"

	query := u.Query()

	if model := r.URL.Query().Get("model"); model != "" {
		query.Set("model", model)
	}

	u.RawQuery = query.Encode()

	headers := http.Header{}

	if h.apiKey != "" {
		headers.Set("Authorization", "Bearer "+h.apiKey)
	}

	dialer := websocket.Dialer{}

	return dialer.Dial(u.String(), headers)
}

func (h *Handler) handleRealtime(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	downstream, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	defer downstream.Close()

	upstream, resp, err := h.dial(r)

	if err != nil {
		log.Printf("Failed to connect to upstream: %v", err)

		if resp != nil {
			data, _ := io.ReadAll(resp.Body)
			log.Print(string(data))
		}

		return
	}

	defer upstream.Close()

	go func() {
		defer cancel()

		for {
			messageType, message, err := downstream.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Client connection error: %v", err)
				}

				return
			}

			if err := upstream.WriteMessage(messageType, message); err != nil {
				log.Printf("Failed to write to upstream: %v", err)
				return
			}
		}
	}()

	go func() {
		defer cancel()

		for {
			messageType, message, err := upstream.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Upstream connection error: %v", err)
				}

				return
			}

			if err := downstream.WriteMessage(messageType, message); err != nil {
				log.Printf("Failed to write to client: %v", err)
				return
			}
		}
	}()

	<-ctx.Done()
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
