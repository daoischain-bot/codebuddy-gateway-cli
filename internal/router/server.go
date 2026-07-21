package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"codebuddy-router/internal/config"
)

type Server struct {
	cfg    *config.Config
	pool   *KeyPool
	client *http.Client
	events chan LogEntry
	mux    *http.ServeMux
}

func NewServer(cfg *config.Config, pool *KeyPool, transport *http.Transport, events chan LogEntry) *Server {
	client := &http.Client{
		Transport: transport,
		Timeout:   120 * time.Second,
	}
	s := &Server{
		cfg:    cfg,
		pool:   pool,
		client: client,
		events: events,
		mux:    http.NewServeMux(),
	}
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/v1/models", s.handleModels)
	s.mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// sendLog: non-blocking send to events channel.
// If TUI is slow/full, drop the log entry rather than blocking the request.
func (s *Server) sendLog(entry LogEntry) {
	select {
	case s.events <- entry:
	default:
		// channel full, drop log (request continues normally)
	}
}

func (s *Server) ListenAndServe() error {
	addr := ":" + s.cfg.Port
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"keys":   len(s.cfg.APIKeys),
		"models": len(Models),
	})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	models := make([]map[string]interface{}, 0, len(Models))
	for _, m := range Models {
		models = append(models, map[string]interface{}{
			"id":                m.ID,
			"object":            "model",
			"created":           1700000000,
			"owned_by":          "codebuddy",
			"name":              m.Name,
			"max_input_tokens":  m.MaxInput,
			"max_output_tokens": m.MaxOutput,
		})
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data":   models,
	})
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Auth check — validate router API key
	if !s.authenticate(r) {
		s.sendLog(LogEntry{
			Time:       time.Now(),
			Method:     r.Method,
			Path:       r.URL.Path,
			Error:      "unauthorized: invalid router key",
			StatusCode: 401,
		})
		http.Error(w, `{"error":"unauthorized: invalid API key"}`, http.StatusUnauthorized)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	start := time.Now()

	var req ChatRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	modelID := StripPrefix(req.Model)

	// Inject system message if missing
	var msgs []map[string]interface{}
	if err := json.Unmarshal(req.Messages, &msgs); err == nil {
		hasSystem := false
		for _, m := range msgs {
			if m["role"] == "system" {
				hasSystem = true
				break
			}
		}
		if !hasSystem {
			msgs = append([]map[string]interface{}{
				{"role": "system", "content": "You are a helpful AI assistant."},
			}, msgs...)
			req.Messages, _ = json.Marshal(msgs)
		}
	}

	req.Model = modelID

	// Key rotation: try every available key exactly once before giving up.
	maxRetries := len(s.cfg.APIKeys)
	if maxRetries == 0 {
		maxRetries = 1
	}

	var lastStatus int
	var lastBody []byte
	for attempt := 0; attempt < maxRetries; attempt++ {
		key := s.pool.Next()
		if key == nil {
			http.Error(w, `{"error":"no upstream keys"}`, http.StatusInternalServerError)
			return
		}

		body, err := BuildUpstreamRequest(&req)
		if err != nil {
			http.Error(w, `{"error":"build failed"}`, http.StatusInternalServerError)
			return
		}

		resp, err := FetchUpstream(s.client, key.Key, UpstreamBase+"/chat/completions", body)
		if err != nil {
			// Network error — always retry with next key
			s.pool.MarkFailed(key, err.Error(), true)
			s.sendLog(LogEntry{
				Time:       time.Now(),
				Method:     r.Method,
				Path:       r.URL.Path,
				Model:      modelID,
				StatusCode: 0,
				Latency:    time.Since(start),
				KeyIndex:   key.Index,
				Error:      fmt.Sprintf("network err — retry %d/%d: %v", attempt+1, maxRetries, err),
			})
			continue
		}

		// Success (2xx) — passthrough stream
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			s.sendLog(LogEntry{
				Time:       time.Now(),
				Method:     r.Method,
				Path:       r.URL.Path,
				Model:      modelID,
				StatusCode: resp.StatusCode,
				Latency:    time.Since(start),
				KeyIndex:   key.Index,
			})
			StreamPassthrough(w, resp)
			resp.Body.Close()
			return
		}

		// Read error body for logging & possible passthrough
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		lastStatus = resp.StatusCode
		lastBody = errBody

		// Retryable (401/403/429/5xx) — mark failed & try next key
		if IsRetryableStatus(resp.StatusCode) {
			s.pool.MarkFailed(key, fmt.Sprintf("HTTP %d", resp.StatusCode), true)
			s.sendLog(LogEntry{
				Time:       time.Now(),
				Method:     r.Method,
				Path:       r.URL.Path,
				Model:      modelID,
				StatusCode: resp.StatusCode,
				Latency:    time.Since(start),
				KeyIndex:   key.Index,
				Error:      fmt.Sprintf("retry %d/%d", attempt+1, maxRetries),
			})
			continue
		}

		// Non-retryable client error (400, 404, etc) — return immediately
		s.sendLog(LogEntry{
			Time:       time.Now(),
			Method:     r.Method,
			Path:       r.URL.Path,
			Model:      modelID,
			StatusCode: resp.StatusCode,
			Latency:    time.Since(start),
			KeyIndex:   key.Index,
			Error:      string(errBody),
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(errBody)
		return
	}

	// All keys exhausted — return upstream last error (usually 429)
	s.sendLog(LogEntry{
		Time:       time.Now(),
		Method:     r.Method,
		Path:       r.URL.Path,
		Model:      modelID,
		StatusCode: lastStatus,
		Latency:    time.Since(start),
		Error:      "all keys exhausted",
	})

	if lastStatus > 0 && len(lastBody) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(lastStatus)
		w.Write(lastBody)
		return
	}
	http.Error(w, `{"error":"all keys exhausted"}`, http.StatusBadGateway)
}

func (s *Server) authenticate(r *http.Request) bool {
	// RouterKey is deterministic per-machine
	if s.cfg.RouterKey == "" {
		return true
	}

	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false
	}

	key := strings.TrimPrefix(auth, "Bearer ")
	key = strings.TrimSpace(key)

	return key == s.cfg.RouterKey
}
