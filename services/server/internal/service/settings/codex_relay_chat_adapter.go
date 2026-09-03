package settings

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type codexResponsesRequest struct {
	Model             string            `json:"model"`
	Instructions      string            `json:"instructions,omitempty"`
	Input             json.RawMessage   `json:"input"`
	Tools             []json.RawMessage `json:"tools,omitempty"`
	ToolChoice        json.RawMessage   `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool             `json:"parallel_tool_calls,omitempty"`
	Temperature       *float64          `json:"temperature,omitempty"`
	TopP              *float64          `json:"top_p,omitempty"`
	MaxOutputTokens   *int              `json:"max_output_tokens,omitempty"`
	Stream            bool              `json:"stream,omitempty"`
}

type codexChatRequest struct {
	Model             string             `json:"model"`
	Messages          []codexChatMessage `json:"messages"`
	Tools             []codexChatTool    `json:"tools,omitempty"`
	ToolChoice        any                `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool              `json:"parallel_tool_calls,omitempty"`
	Temperature       *float64           `json:"temperature,omitempty"`
	TopP              *float64           `json:"top_p,omitempty"`
	MaxTokens         *int               `json:"max_tokens,omitempty"`
	Stream            bool               `json:"stream"`
}

type codexChatMessage struct {
	Role       string              `json:"role"`
	Content    any                 `json:"content,omitempty"`
	ToolCalls  []codexChatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
}

type codexChatTool struct {
	Type     string                `json:"type"`
	Function codexChatToolFunction `json:"function"`
}

type codexChatToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type codexChatToolCall struct {
	ID       string                `json:"id"`
	Type     string                `json:"type"`
	Function codexChatFunctionCall `json:"function"`
}

type codexChatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type codexChatResponse struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string              `json:"role"`
			Content   any                 `json:"content"`
			ToolCalls []codexChatToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func codexRelayResponsesToChatRequest(body []byte, fallbackModel string) ([]byte, bool, error) {
	var source codexResponsesRequest
	if err := json.Unmarshal(body, &source); err != nil {
		return nil, false, fmt.Errorf("%w: invalid Responses request: %v", ErrCodexRelayInvalid, err)
	}
	model := strings.TrimSpace(source.Model)
	if model == "" {
		model = strings.TrimSpace(fallbackModel)
	}
	if model == "" {
		return nil, false, fmt.Errorf("%w: model is required", ErrCodexRelayInvalid)
	}
	messages, err := codexResponsesInputToChatMessages(source.Instructions, source.Input)
	if err != nil {
		return nil, false, err
	}
	tools, err := codexResponsesToolsToChat(source.Tools)
	if err != nil {
		return nil, false, err
	}
	toolChoice, err := codexResponsesToolChoiceToChat(source.ToolChoice)
	if err != nil {
		return nil, false, err
	}
	payload := codexChatRequest{
		Model:             model,
		Messages:          messages,
		Tools:             tools,
		ToolChoice:        toolChoice,
		ParallelToolCalls: source.ParallelToolCalls,
		Temperature:       source.Temperature,
		TopP:              source.TopP,
		MaxTokens:         source.MaxOutputTokens,
		Stream:            false,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, false, fmt.Errorf("encoding Chat Completions request: %w", err)
	}
	return encoded, source.Stream, nil
}

func codexResponsesInputToChatMessages(instructions string, raw json.RawMessage) ([]codexChatMessage, error) {
	messages := make([]codexChatMessage, 0, 8)
	if text := strings.TrimSpace(instructions); text != "" {
		messages = append(messages, codexChatMessage{Role: "system", Content: text})
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return messages, nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		messages = append(messages, codexChatMessage{Role: "user", Content: text})
		return messages, nil
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("%w: unsupported Responses input", ErrCodexRelayInvalid)
	}
	for _, item := range items {
		typeName := stringMapValue(item, "type")
		switch typeName {
		case "message", "":
			role := stringMapValue(item, "role")
			if role == "" {
				role = "user"
			}
			content := codexResponsesContentToChat(item["content"])
			messages = append(messages, codexChatMessage{Role: role, Content: content})
		case "function_call":
			callID := firstNonEmpty(stringMapValue(item, "call_id"), stringMapValue(item, "id"))
			messages = append(messages, codexChatMessage{
				Role: "assistant",
				ToolCalls: []codexChatToolCall{{
					ID:   callID,
					Type: "function",
					Function: codexChatFunctionCall{
						Name:      stringMapValue(item, "name"),
						Arguments: firstNonEmpty(stringMapValue(item, "arguments"), "{}"),
					},
				}},
			})
		case "function_call_output":
			messages = append(messages, codexChatMessage{
				Role:       "tool",
				ToolCallID: stringMapValue(item, "call_id"),
				Content:    codexResponseOutputString(item["output"]),
			})
		case "reasoning":
			// Reasoning items are model-internal state. Chat Completions cannot replay them.
		default:
			return nil, fmt.Errorf("%w: unsupported Responses input item type %q", ErrCodexRelayInvalid, typeName)
		}
	}
	return messages, nil
}

func codexResponsesContentToChat(value any) any {
	items, ok := value.([]any)
	if !ok {
		if value == nil {
			return ""
		}
		return value
	}
	parts := make([]map[string]any, 0, len(items))
	var textOnly strings.Builder
	allText := true
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typeName := stringMapValue(item, "type")
		switch typeName {
		case "input_text", "output_text", "text":
			text := stringMapValue(item, "text")
			textOnly.WriteString(text)
			parts = append(parts, map[string]any{"type": "text", "text": text})
		case "input_image", "image_url":
			allText = false
			imageURL := stringMapValue(item, "image_url")
			if imageURL == "" {
				imageURL = stringMapValue(item, "url")
			}
			if imageURL != "" {
				parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": imageURL}})
			}
		default:
			allText = false
		}
	}
	if allText {
		return textOnly.String()
	}
	return parts
}

func codexResponsesToolsToChat(rawTools []json.RawMessage) ([]codexChatTool, error) {
	tools := make([]codexChatTool, 0, len(rawTools))
	for _, raw := range rawTools {
		var item map[string]any
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("%w: invalid tool definition", ErrCodexRelayInvalid)
		}
		if stringMapValue(item, "type") != "function" {
			continue
		}
		name := stringMapValue(item, "name")
		if name == "" {
			if fn, ok := item["function"].(map[string]any); ok {
				name = stringMapValue(fn, "name")
				item = fn
			}
		}
		if name == "" {
			return nil, fmt.Errorf("%w: function tool name is required", ErrCodexRelayInvalid)
		}
		tool := codexChatTool{Type: "function", Function: codexChatToolFunction{
			Name:        name,
			Description: stringMapValue(item, "description"),
			Parameters:  codexOpenAICompatibleToolParameters(item["parameters"]),
		}}
		tools = append(tools, tool)
	}
	return tools, nil
}

func codexOpenAICompatibleToolParameters(value any) map[string]any {
	parameters, ok := value.(map[string]any)
	if !ok || parameters == nil {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}
	result := sanitizeOpenAICompatibleSchemaMap(parameters)
	if stringMapValue(result, "type") == "" {
		result["type"] = "object"
	}
	if _, ok := result["properties"]; !ok {
		result["properties"] = map[string]any{}
	}
	return result
}

func sanitizeOpenAICompatibleSchemaMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		switch key {
		case "$schema", "$id":
			continue
		}
		result[key] = sanitizeOpenAICompatibleSchemaValue(value)
	}
	return result
}

func sanitizeOpenAICompatibleSchemaValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return sanitizeOpenAICompatibleSchemaMap(typed)
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = sanitizeOpenAICompatibleSchemaValue(item)
		}
		return result
	default:
		return value
	}
}

func codexResponsesToolChoiceToChat(raw json.RawMessage) (any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var text string
	if json.Unmarshal(trimmed, &text) == nil {
		switch text {
		case "auto", "none", "required":
			return text, nil
		default:
			return nil, nil
		}
	}
	var object map[string]any
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return nil, fmt.Errorf("%w: invalid tool_choice", ErrCodexRelayInvalid)
	}
	if stringMapValue(object, "type") == "function" {
		name := stringMapValue(object, "name")
		if name == "" {
			if fn, ok := object["function"].(map[string]any); ok {
				name = stringMapValue(fn, "name")
			}
		}
		if name != "" {
			return map[string]any{"type": "function", "function": map[string]any{"name": name}}, nil
		}
	}
	return nil, nil
}

func codexRelayChatResponseToResponses(upstream *http.Response, stream bool) (*http.Response, error) {
	if upstream == nil {
		return nil, fmt.Errorf("empty Chat Completions response")
	}
	if upstream.StatusCode < 200 || upstream.StatusCode >= 300 {
		return upstream, nil
	}
	body, err := io.ReadAll(io.LimitReader(upstream.Body, 32<<20))
	upstream.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("reading Chat Completions response: %w", err)
	}
	var chat codexChatResponse
	if err := json.Unmarshal(body, &chat); err != nil {
		return nil, fmt.Errorf("decoding Chat Completions response: %w", err)
	}
	response := codexChatToResponsesObject(chat)
	var encoded []byte
	contentType := "application/json"
	if stream {
		encoded, err = codexResponsesSSE(response)
		contentType = "text/event-stream"
	} else {
		encoded, err = json.Marshal(response)
	}
	if err != nil {
		return nil, err
	}
	result := &http.Response{
		StatusCode: upstream.StatusCode,
		Status:     upstream.Status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(encoded)),
		Request:    upstream.Request,
	}
	result.Header.Set("Content-Type", contentType)
	if stream {
		result.Header.Set("Cache-Control", "no-cache")
	}
	return result, nil
}

func codexChatToResponsesObject(chat codexChatResponse) map[string]any {
	id := strings.TrimSpace(chat.ID)
	if id == "" {
		id = "resp_chat_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	} else if !strings.HasPrefix(id, "resp_") {
		id = "resp_" + id
	}
	created := chat.Created
	if created == 0 {
		created = time.Now().Unix()
	}
	output := make([]map[string]any, 0, 4)
	if len(chat.Choices) > 0 {
		choice := chat.Choices[0]
		if text := codexChatContentString(choice.Message.Content); text != "" {
			output = append(output, map[string]any{
				"id":     "msg_" + id,
				"type":   "message",
				"status": "completed",
				"role":   "assistant",
				"content": []map[string]any{{
					"type":        "output_text",
					"text":        text,
					"annotations": []any{},
				}},
			})
		}
		for index, call := range choice.Message.ToolCalls {
			callID := strings.TrimSpace(call.ID)
			if callID == "" {
				callID = fmt.Sprintf("call_%s_%d", id, index)
			}
			output = append(output, map[string]any{
				"id":        fmt.Sprintf("fc_%s_%d", id, index),
				"type":      "function_call",
				"status":    "completed",
				"call_id":   callID,
				"name":      call.Function.Name,
				"arguments": firstNonEmpty(call.Function.Arguments, "{}"),
			})
		}
	}
	return map[string]any{
		"id":                  id,
		"object":              "response",
		"created_at":          created,
		"status":              "completed",
		"model":               chat.Model,
		"output":              output,
		"parallel_tool_calls": true,
		"usage": map[string]any{
			"input_tokens":  chat.Usage.PromptTokens,
			"output_tokens": chat.Usage.CompletionTokens,
			"total_tokens":  chat.Usage.TotalTokens,
		},
	}
}

func codexResponsesSSE(response map[string]any) ([]byte, error) {
	var buffer bytes.Buffer
	writer := bufio.NewWriter(&buffer)
	sequence := 0
	emit := func(event string, payload map[string]any) error {
		payload["type"] = event
		payload["sequence_number"] = sequence
		sequence++
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, encoded); err != nil {
			return err
		}
		return nil
	}
	if err := emit("response.created", map[string]any{"response": response}); err != nil {
		return nil, err
	}
	output, _ := response["output"].([]map[string]any)
	for index, item := range output {
		if err := emit("response.output_item.added", map[string]any{"output_index": index, "item": item}); err != nil {
			return nil, err
		}
		switch item["type"] {
		case "message":
			content, _ := item["content"].([]map[string]any)
			for contentIndex, part := range content {
				if err := emit("response.content_part.added", map[string]any{"item_id": item["id"], "output_index": index, "content_index": contentIndex, "part": part}); err != nil {
					return nil, err
				}
				text, _ := part["text"].(string)
				if text != "" {
					if err := emit("response.output_text.delta", map[string]any{"item_id": item["id"], "output_index": index, "content_index": contentIndex, "delta": text}); err != nil {
						return nil, err
					}
				}
				if err := emit("response.output_text.done", map[string]any{"item_id": item["id"], "output_index": index, "content_index": contentIndex, "text": text}); err != nil {
					return nil, err
				}
				if err := emit("response.content_part.done", map[string]any{"item_id": item["id"], "output_index": index, "content_index": contentIndex, "part": part}); err != nil {
					return nil, err
				}
			}
		case "function_call":
			arguments, _ := item["arguments"].(string)
			if arguments != "" {
				if err := emit("response.function_call_arguments.delta", map[string]any{"item_id": item["id"], "output_index": index, "delta": arguments}); err != nil {
					return nil, err
				}
			}
			if err := emit("response.function_call_arguments.done", map[string]any{"item_id": item["id"], "output_index": index, "arguments": arguments}); err != nil {
				return nil, err
			}
		}
		if err := emit("response.output_item.done", map[string]any{"output_index": index, "item": item}); err != nil {
			return nil, err
		}
	}
	if err := emit("response.completed", map[string]any{"response": response}); err != nil {
		return nil, err
	}
	if err := writer.Flush(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func codexChatContentString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		var builder strings.Builder
		for _, raw := range typed {
			part, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if text := stringMapValue(part, "text"); text != "" {
				builder.WriteString(text)
			}
		}
		return builder.String()
	default:
		return ""
	}
}

func codexResponseOutputString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(encoded)
}

func stringMapValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
