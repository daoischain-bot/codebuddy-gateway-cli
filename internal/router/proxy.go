package router

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const UpstreamBase = "https://www.codebuddy.ai/v2"

var SpoofHeaders = map[string]string{
	"User-Agent":                "CLI/2.105.2 CodeBuddy/2.105.2",
	"X-Product":                 "SaaS",
	"X-App":                     "cli",
	"X-Stainless-Runtime":       "node",
	"X-Stainless-Lang":          "js",
	"X-Stainless-Helper-Method": "stream",
	"X-IDE-Type":                "CLI",
	"X-IDE-Name":                "CLI",
	"X-IDE-Version":             "2.105.2",
	"X-Private-Data":            "false",
	"X-Requested-With":          "XMLHttpRequest",
	"X-Domain":                  "www.codebuddy.ai",
	"Origin":                    "https://www.codebuddy.ai",
	"Referer":                   "https://www.codebuddy.ai/",
}

type ChatRequest struct {
	Model       string          `json:"model"`
	Messages    json.RawMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
}

type SSEChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

type Choice struct {
	Index        int         `json:"index"`
	Delta        Delta       `json:"delta"`
	FinishReason *string     `json:"finish_reason,omitempty"`
	Logprobs     interface{} `json:"logprobs,omitempty"`
}

type Delta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	Index    *int            `json:"index,omitempty"`
	ID       string          `json:"id,omitempty"`
	Type     string          `json:"type,omitempty"`
	Function ToolCallFunction `json:"function,omitempty"`
}

type ToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type AggregatedResponse struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int64       `json:"created"`
	Model   string      `json:"model"`
	Choices []AggChoice `json:"choices"`
	Usage   Usage       `json:"usage"`
}

type AggChoice struct {
	Index        int         `json:"index"`
	Message      AggMessage  `json:"message"`
	FinishReason string      `json:"finish_reason"`
	Logprobs     interface{} `json:"logprobs,omitempty"`
}

type AggMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type LogEntry struct {
	Time       time.Time
	Method     string
	Path       string
	Model      string
	StatusCode int
	Latency    time.Duration
	Tokens     int
	KeyIndex   int
	Error      string
}

func BuildUpstreamRequest(req *ChatRequest) ([]byte, error) {
	if req.MaxTokens != nil && *req.MaxTokens > 32000 {
		t := 32000
		req.MaxTokens = &t
	}
	req.Stream = true

	body := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}
	if req.MaxTokens != nil {
		body["max_tokens"] = *req.MaxTokens
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	return json.Marshal(body)
}

func DetermineTransport(proxyURL string) (*http.Transport, error) {
	transport := &http.Transport{
		MaxIdleConns:      100,
		IdleConnTimeout:   90 * time.Second,
		DisableKeepAlives: false,
	}
	if proxyURL == "" {
		return transport, nil
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	switch u.Scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(u)
	case "socks5", "socks5h":
		dialer, err := proxy.FromURL(u, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("socks5: %w", err)
		}
		nd := net.Dialer{Timeout: 30 * time.Second}
		transport.DialContext = nd.DialContext
		transport.Dial = dialer.Dial
	default:
		return nil, fmt.Errorf("unsupported proxy: %s", u.Scheme)
	}
	return transport, nil
}

func ApplySpoofHeaders(h http.Header) {
	for k, v := range SpoofHeaders {
		h.Set(k, v)
	}
}

func FetchUpstream(client *http.Client, apiKey, url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	ApplySpoofHeaders(req.Header)
	return client.Do(req)
}

func StreamPassthrough(w http.ResponseWriter, resp *http.Response) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, resp.Body)
		return
	}
	buf := bufio.NewReader(resp.Body)
	for {
		line, err := buf.ReadBytes('\n')
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				w.Write(line)
				flusher.Flush()
			}
			return
		}
		w.Write(line)
		flusher.Flush()
	}
}

func AggregateStream(body io.Reader) (AggregatedResponse, error) {
	var agg AggregatedResponse
	agg.Object = "chat.completion"
	agg.ID = fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli())
	agg.Created = time.Now().Unix()

	scanner := bufio.NewScanner(body)
	var contentBuf strings.Builder
	finishReason := "stop"
	toolCallMap := make(map[int]ToolCall)
	var finalUsage Usage

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var chunk SSEChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if agg.Model == "" && chunk.Model != "" {
			agg.Model = chunk.Model
		}
		for _, c := range chunk.Choices {
			if c.FinishReason != nil && *c.FinishReason != "" {
				finishReason = *c.FinishReason
			}
			if c.Delta.Content != "" {
				contentBuf.WriteString(c.Delta.Content)
			}
			for _, tc := range c.Delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				existing := toolCallMap[idx]
				if tc.ID != "" {
					existing.ID = tc.ID
				}
				existing.Type = "function"
				if tc.Function.Name != "" {
					existing.Function.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					existing.Function.Arguments += tc.Function.Arguments
				}
				toolCallMap[idx] = existing
			}
		}
		if chunk.Usage != nil {
			finalUsage = *chunk.Usage
		}
	}

	var toolCalls []ToolCall
	for i := 0; i < len(toolCallMap); i++ {
		tc := toolCallMap[i]
		if tc.Function.Name != "" {
			toolCalls = append(toolCalls, tc)
		}
	}

	msg := AggMessage{Role: "assistant", Content: contentBuf.String()}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
		if finishReason == "stop" {
			finishReason = "tool_calls"
		}
	}

	agg.Choices = []AggChoice{{
		Index: 0, Message: msg, FinishReason: finishReason, Logprobs: nil,
	}}
	agg.Usage = finalUsage
	return agg, nil
}
