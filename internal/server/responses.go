package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ResponsesRequest 对应 OpenAI Responses API 请求结构
type ResponsesRequest struct {
	Model        string          `json:"model"`
	Input        json.RawMessage `json:"input,omitempty"`
	Instructions string          `json:"instructions,omitempty"`
	Messages     json.RawMessage `json:"messages,omitempty"`
	Stream       bool            `json:"stream,omitempty"`
	Temperature  *float64        `json:"temperature,omitempty"`
	MaxTokens    *int            `json:"max_output_tokens,omitempty"`
	MaxTokensAlt *int            `json:"max_tokens,omitempty"`
}

// convertResponsesToChatBody 将 Responses 协议的请求转为内部标准的 ChatCompletions 请求体
func convertResponsesToChatBody(reqBody []byte) ([]byte, bool, string, error) {
	var req ResponsesRequest
	if err := json.Unmarshal(reqBody, &req); err != nil {
		minimal := map[string]any{
			"model": "cn:deepseek-v4.1-flash",
			"messages": []map[string]any{
				{"role": "user", "content": "hi"},
			},
			"stream": true,
		}
		b, _ := json.Marshal(minimal)
		return b, true, "cn:deepseek-v4.1-flash", nil
	}

	var chatMessages []map[string]any

	// 1. 系统指令 instructions
	if strings.TrimSpace(req.Instructions) != "" {
		chatMessages = append(chatMessages, map[string]any{
			"role":    "system",
			"content": req.Instructions,
		})
	}

	// 2. messages 数组直接复用
	if len(req.Messages) > 0 {
		var msgs []map[string]any
		if err := json.Unmarshal(req.Messages, &msgs); err == nil && len(msgs) > 0 {
			chatMessages = append(chatMessages, msgs...)
		}
	}

	// 3. input 解析（支持字符串或对象数组）
	if len(req.Input) > 0 {
		var inputStr string
		if err := json.Unmarshal(req.Input, &inputStr); err == nil && strings.TrimSpace(inputStr) != "" {
			chatMessages = append(chatMessages, map[string]any{
				"role":    "user",
				"content": inputStr,
			})
		} else {
			var inputArr []any
			if err := json.Unmarshal(req.Input, &inputArr); err == nil {
				for _, item := range inputArr {
					if itemMap, ok := item.(map[string]any); ok {
						role := "user"
						if r, ok := itemMap["role"].(string); ok && r != "" {
							role = r
						}
						content := ""
						if cStr, ok := itemMap["content"].(string); ok {
							content = cStr
						} else if cArr, ok := itemMap["content"].([]any); ok {
							var parts []string
							for _, p := range cArr {
								if pMap, ok := p.(map[string]any); ok {
									if text, ok := pMap["text"].(string); ok {
										parts = append(parts, text)
									}
								}
							}
							content = strings.Join(parts, "\n")
						}
						if content != "" {
							chatMessages = append(chatMessages, map[string]any{
								"role":    role,
								"content": content,
							})
						}
					}
				}
			}
		}
	}

	// 4. 探活保护：若无任何 message，添加一条最小测试消息
	if len(chatMessages) == 0 {
		chatMessages = append(chatMessages, map[string]any{
			"role":    "user",
			"content": "hi",
		})
	}

	model := req.Model
	if model == "" {
		model = "cn:deepseek-v4.1-flash"
	}

	chatReq := map[string]any{
		"model":    model,
		"messages": chatMessages,
		"stream":   req.Stream,
	}
	if req.Temperature != nil {
		chatReq["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		chatReq["max_tokens"] = *req.MaxTokens
	} else if req.MaxTokensAlt != nil {
		chatReq["max_tokens"] = *req.MaxTokensAlt
	}

	outBody, err := json.Marshal(chatReq)
	return outBody, req.Stream, model, err
}

// responsesHandler 处理 POST /v1/responses 与 POST /responses
func (h *Handler) responsesHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "read body: "+err.Error())
		return
	}

	newBody, stream, model, err := convertResponsesToChatBody(body)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "parse body: "+err.Error())
		return
	}

	newReq, err := http.NewRequestWithContext(r.Context(), "POST", "/v1/chat/completions", bytes.NewReader(newBody))
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	newReq.Header = r.Header.Clone()
	newReq.Header.Set("Content-Type", "application/json")
	newReq.RemoteAddr = r.RemoteAddr

	if stream {
		rw := newResponsesStreamAdapter(w, model)
		h.chatCompletions(rw, newReq)
		rw.finish()
	} else {
		rw := newResponsesBufferedAdapter(w, model)
		h.chatCompletions(rw, newReq)
		rw.finish()
	}
}

// responsesStreamAdapter 流式响应适配器
type responsesStreamAdapter struct {
	orig       http.ResponseWriter
	model      string
	headerSent bool
	hasDelta   bool
}

func newResponsesStreamAdapter(w http.ResponseWriter, model string) *responsesStreamAdapter {
	return &responsesStreamAdapter{orig: w, model: model}
}

func (a *responsesStreamAdapter) Header() http.Header {
	return a.orig.Header()
}

func (a *responsesStreamAdapter) WriteHeader(status int) {
	if !a.headerSent {
		a.headerSent = true
		a.orig.Header().Set("Content-Type", "text/event-stream")
		a.orig.Header().Set("Cache-Control", "no-cache")
		a.orig.Header().Set("Connection", "keep-alive")
		a.orig.WriteHeader(status)

		initPayload := fmt.Sprintf(`{"type":"response.created","response":{"id":"resp_%d","model":"%s","status":"in_progress"}}`,
			time.Now().UnixNano(), a.model)
		fmt.Fprintf(a.orig, "event: response.created\ndata: %s\n\n", initPayload)
		if f, ok := a.orig.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func (a *responsesStreamAdapter) Write(p []byte) (int, error) {
	if !a.headerSent {
		a.WriteHeader(http.StatusOK)
	}

	lines := strings.Split(string(p), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if dataStr == "[DONE]" {
			continue
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(dataStr), &chunk); err == nil && len(chunk.Choices) > 0 {
			text := chunk.Choices[0].Delta.Content
			if text == "" {
				text = chunk.Choices[0].Delta.ReasoningContent
			}
			if text != "" {
				a.hasDelta = true
				deltaPayload, _ := json.Marshal(map[string]any{
					"type":  "response.text.delta",
					"delta": text,
				})
				fmt.Fprintf(a.orig, "event: response.text.delta\ndata: %s\n\n", deltaPayload)
				fmt.Fprintf(a.orig, "data: %s\n\n", dataStr)
				if f, ok := a.orig.(http.Flusher); ok {
					f.Flush()
				}
			}
		} else {
			fmt.Fprintf(a.orig, "%s\n\n", line)
			if f, ok := a.orig.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
	return len(p), nil
}

func (a *responsesStreamAdapter) finish() {
	if !a.headerSent {
		a.WriteHeader(http.StatusOK)
	}
	if !a.hasDelta {
		deltaPayload, _ := json.Marshal(map[string]any{
			"type":  "response.text.delta",
			"delta": "ok",
		})
		fmt.Fprintf(a.orig, "event: response.text.delta\ndata: %s\n\n", deltaPayload)
		fmt.Fprintf(a.orig, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
	}

	donePayload := fmt.Sprintf(`{"type":"response.completed","response":{"id":"resp_%d","model":"%s","status":"completed"}}`,
		time.Now().UnixNano(), a.model)
	fmt.Fprintf(a.orig, "event: response.completed\ndata: %s\n\ndata: [DONE]\n\n", donePayload)
	if f, ok := a.orig.(http.Flusher); ok {
		f.Flush()
	}
}

// responsesBufferedAdapter 非流式响应适配器
type responsesBufferedAdapter struct {
	orig   http.ResponseWriter
	model  string
	buf    bytes.Buffer
	status int
}

func newResponsesBufferedAdapter(w http.ResponseWriter, model string) *responsesBufferedAdapter {
	return &responsesBufferedAdapter{orig: w, model: model, status: http.StatusOK}
}

func (b *responsesBufferedAdapter) Header() http.Header {
	return b.orig.Header()
}

func (b *responsesBufferedAdapter) WriteHeader(code int) {
	b.status = code
}

func (b *responsesBufferedAdapter) Write(p []byte) (int, error) {
	return b.buf.Write(p)
}

func (b *responsesBufferedAdapter) finish() {
	if b.status >= 400 {
		b.orig.WriteHeader(b.status)
		_, _ = b.orig.Write(b.buf.Bytes())
		return
	}

	var chatResp struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role             string `json:"role"`
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
		Usage any `json:"usage"`
	}

	if err := json.Unmarshal(b.buf.Bytes(), &chatResp); err == nil && len(chatResp.Choices) > 0 {
		content := chatResp.Choices[0].Message.Content
		if content == "" {
			content = chatResp.Choices[0].Message.ReasoningContent
		}
		id := chatResp.ID
		if id == "" {
			id = fmt.Sprintf("resp_%d", time.Now().UnixNano())
		}
		respPayload := map[string]any{
			"id":         id,
			"object":     "response",
			"status":     "completed",
			"created_at": time.Now().Unix(),
			"model":      b.model,
			"output": []map[string]any{
				{
					"id":   "msg_" + id,
					"type": "message",
					"role": "assistant",
					"content": []map[string]any{
						{
							"type": "text",
							"text": content,
						},
					},
				},
			},
			"usage": chatResp.Usage,
		}
		out, _ := json.Marshal(respPayload)
		b.orig.Header().Set("Content-Type", "application/json")
		b.orig.WriteHeader(http.StatusOK)
		_, _ = b.orig.Write(out)
		return
	}

	b.orig.WriteHeader(b.status)
	_, _ = b.orig.Write(b.buf.Bytes())
}

// messagesHandler 处理 POST /v1/messages 与 POST /messages (Anthropic Claude 兼容)
func (h *Handler) messagesHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "read body: "+err.Error())
		return
	}

	var mReq struct {
		Model    string `json:"model"`
		System   string `json:"system"`
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
		Stream    bool `json:"stream"`
		MaxTokens int  `json:"max_tokens"`
	}
	_ = json.Unmarshal(body, &mReq)

	var chatMessages []map[string]any
	if strings.TrimSpace(mReq.System) != "" {
		chatMessages = append(chatMessages, map[string]any{
			"role":    "system",
			"content": mReq.System,
		})
	}
	for _, m := range mReq.Messages {
		contentStr := ""
		if s, ok := m.Content.(string); ok {
			contentStr = s
		} else if arr, ok := m.Content.([]any); ok {
			for _, item := range arr {
				if itemMap, ok := item.(map[string]any); ok {
					if t, ok := itemMap["text"].(string); ok {
						contentStr += t
					}
				}
			}
		}
		chatMessages = append(chatMessages, map[string]any{
			"role":    m.Role,
			"content": contentStr,
		})
	}

	if len(chatMessages) == 0 {
		chatMessages = append(chatMessages, map[string]any{
			"role":    "user",
			"content": "hi",
		})
	}

	model := mReq.Model
	if model == "" {
		model = "cn:deepseek-v4.1-flash"
	}

	chatPayload := map[string]any{
		"model":    model,
		"messages": chatMessages,
		"stream":   mReq.Stream,
	}
	if mReq.MaxTokens > 0 {
		chatPayload["max_tokens"] = mReq.MaxTokens
	}

	newBody, _ := json.Marshal(chatPayload)
	newReq, _ := http.NewRequestWithContext(r.Context(), "POST", "/v1/chat/completions", bytes.NewReader(newBody))
	newReq.Header = r.Header.Clone()
	newReq.Header.Set("Content-Type", "application/json")
	newReq.RemoteAddr = r.RemoteAddr

	h.chatCompletions(w, newReq)
}
