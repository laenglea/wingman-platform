package responses

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Reasoning and hosted tools can leave long gaps between visible events.
// SSE comments keep downstream proxies from closing the idle connection.
// Start only after selecting SSE headers, preserving HTTP errors before output.
func keepResponseStreamAlive(ctx context.Context, cancel context.CancelFunc, w http.ResponseWriter) (http.ResponseWriter, func()) {
	s := &responseStreamWriter{ResponseWriter: w, cancel: cancel}
	ctx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := s.Write([]byte(": keep-alive\n\n")); err != nil {
					return
				}
				if err := s.FlushError(); err != nil {
					return
				}
			}
		}
	}()
	return s, func() {
		stop()
		<-done // Never write to a ResponseWriter after the handler returns.
	}
}

// Serialize provider events and keepalives; ResponseWriter is not safe for
// concurrent use. A write failure also cancels a provider paused in reasoning.
type responseStreamWriter struct {
	http.ResponseWriter
	mu     sync.Mutex
	err    error
	cancel context.CancelFunc
}

func (s *responseStreamWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, s.err
	}
	n, err := s.ResponseWriter.Write(p)
	if err != nil {
		s.err = err
		s.cancel()
	}
	return n, err
}

func (s *responseStreamWriter) FlushError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.err = http.NewResponseController(s.ResponseWriter).Flush()
		if s.err != nil {
			s.cancel()
		}
	}
	return s.err
}
