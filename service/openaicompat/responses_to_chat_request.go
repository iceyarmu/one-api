package openaicompat

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// ResponsesRequestToChatCompletionsRequest converts an OpenAI Responses API request
// to a Chat Completions API request for channels that don't support Responses API natively.
//
// Conversion rules:
// - input → messages (parse JSON array or string)
// - instructions → messages[0] (role: system)
// - max_output_tokens → max_tokens
// - tools (function type) → tools
// - tool_choice → tool_choice
// - reasoning.effort → reasoning_effort
// - temperature, top_p → direct mapping
func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if req.Model == "" {
		return nil, errors.New("model is required")
	}

	messages := make([]dto.Message, 0)

	// Process instructions as system message
	if len(req.Instructions) > 0 {
		var instructions string
		if err := common.Unmarshal(req.Instructions, &instructions); err == nil && strings.TrimSpace(instructions) != "" {
			messages = append(messages, dto.Message{
				Role:    "system",
				Content: instructions,
			})
		}
	}

	// Process input field
	if len(req.Input) > 0 {
		inputMessages, err := parseResponsesInput(req.Input)
		if err != nil {
			return nil, err
		}
		messages = append(messages, inputMessages...)
	}

	// Convert tools
	var tools []dto.ToolCallRequest
	if len(req.Tools) > 0 {
		tools = convertResponsesTools(req.Tools)
	}

	// Convert tool_choice
	var toolChoice any
	if len(req.ToolChoice) > 0 {
		toolChoice = convertResponsesToolChoice(req.ToolChoice)
	}

	// Convert parallel_tool_calls
	var parallelToolCalls *bool
	if len(req.ParallelToolCalls) > 0 {
		var ptc bool
		if err := common.Unmarshal(req.ParallelToolCalls, &ptc); err == nil {
			parallelToolCalls = &ptc
		}
	}

	// Build the Chat Completions request
	chatReq := &dto.GeneralOpenAIRequest{
		Model:            req.Model,
		Messages:         messages,
		Stream:           req.Stream,
		MaxTokens:        req.MaxOutputTokens,
		Temperature:      req.Temperature,
		Tools:            tools,
		ToolChoice:       toolChoice,
		User:             req.User,
		ParallelTooCalls: parallelToolCalls,
		Store:            req.Store,
		Metadata:         req.Metadata,
	}

	// Set TopP only if provided
	if req.TopP != nil {
		chatReq.TopP = *req.TopP
	}

	// Convert reasoning
	if req.Reasoning != nil && req.Reasoning.Effort != "" && req.Reasoning.Effort != "none" {
		chatReq.ReasoningEffort = req.Reasoning.Effort
	}

	return chatReq, nil
}

// parseResponsesInput parses the Responses API input field into Chat Completions messages
func parseResponsesInput(inputRaw []byte) ([]dto.Message, error) {
	if len(inputRaw) == 0 {
		return nil, nil
	}

	messages := make([]dto.Message, 0)

	// Check if input is a string
	if common.GetJsonType(inputRaw) == "string" {
		var str string
		if err := common.Unmarshal(inputRaw, &str); err == nil {
			messages = append(messages, dto.Message{
				Role:    "user",
				Content: str,
			})
			return messages, nil
		}
	}

	// Parse as array
	if common.GetJsonType(inputRaw) != "array" {
		return nil, errors.New("input must be a string or array")
	}

	var inputItems []map[string]any
	if err := common.Unmarshal(inputRaw, &inputItems); err != nil {
		return nil, err
	}

	for _, item := range inputItems {
		itemType, _ := item["type"].(string)
		role, _ := item["role"].(string)

		switch itemType {
		case "message", "":
			// Standard message item
			if role == "" {
				role = "user"
			}
			msg := dto.Message{Role: role}

			// Parse content
			if content, ok := item["content"]; ok {
				msg.Content = convertResponsesContent(content)
			}

			messages = append(messages, msg)

		case "function_call":
			// Function call from assistant - convert to assistant message with tool_calls
			callID, _ := item["call_id"].(string)
			name, _ := item["name"].(string)
			arguments, _ := item["arguments"].(string)

			if callID != "" && name != "" {
				toolCall := dto.ToolCallResponse{
					ID:   callID,
					Type: "function",
					Function: dto.FunctionResponse{
						Name:      name,
						Arguments: arguments,
					},
				}

				// Check if we need to append to existing assistant message or create new one
				lastIdx := len(messages) - 1
				if lastIdx >= 0 && messages[lastIdx].Role == "assistant" {
					// Parse existing tool calls and append new one
					var existingCalls []dto.ToolCallResponse
					if messages[lastIdx].ToolCalls != nil {
						_ = common.Unmarshal(messages[lastIdx].ToolCalls, &existingCalls)
					}
					existingCalls = append(existingCalls, toolCall)
					messages[lastIdx].SetToolCalls(existingCalls)
				} else {
					// Create new assistant message with tool call
					msg := dto.Message{Role: "assistant"}
					msg.SetToolCalls([]dto.ToolCallResponse{toolCall})
					messages = append(messages, msg)
				}
			}

		case "function_call_output":
			// Tool response - convert to tool message
			callID, _ := item["call_id"].(string)
			output, _ := item["output"].(string)

			if callID != "" {
				messages = append(messages, dto.Message{
					Role:       "tool",
					Content:    output,
					ToolCallId: callID,
				})
			}
		}
	}

	return messages, nil
}

// convertResponsesContent converts Responses API content to Chat Completions content format
func convertResponsesContent(content any) any {
	switch c := content.(type) {
	case string:
		return c
	case []any:
		// Array of content parts
		chatParts := make([]map[string]any, 0, len(c))
		for _, part := range c {
			if partMap, ok := part.(map[string]any); ok {
				partType, _ := partMap["type"].(string)
				switch partType {
				case "input_text":
					text, _ := partMap["text"].(string)
					chatParts = append(chatParts, map[string]any{
						"type": "text",
						"text": text,
					})
				case "input_image":
					imageURL := extractImageURL(partMap)
					if imageURL != "" {
						chatParts = append(chatParts, map[string]any{
							"type": "image_url",
							"image_url": map[string]any{
								"url": imageURL,
							},
						})
					}
				case "input_audio":
					if inputAudio, ok := partMap["input_audio"]; ok {
						chatParts = append(chatParts, map[string]any{
							"type":        "input_audio",
							"input_audio": inputAudio,
						})
					}
				case "input_file":
					if file, ok := partMap["file"]; ok {
						chatParts = append(chatParts, map[string]any{
							"type": "file",
							"file": file,
						})
					}
				case "output_text":
					// For assistant messages with output_text
					text, _ := partMap["text"].(string)
					chatParts = append(chatParts, map[string]any{
						"type": "text",
						"text": text,
					})
				default:
					// Keep original for unknown types
					chatParts = append(chatParts, partMap)
				}
			}
		}
		if len(chatParts) > 0 {
			return chatParts
		}
		return ""
	default:
		return ""
	}
}

// extractImageURL extracts image URL from various formats
func extractImageURL(partMap map[string]any) string {
	if imageURL, ok := partMap["image_url"].(string); ok {
		return imageURL
	}
	if imageURLMap, ok := partMap["image_url"].(map[string]any); ok {
		if url, ok := imageURLMap["url"].(string); ok {
			return url
		}
	}
	return ""
}

// convertResponsesTools converts Responses API tools to Chat Completions tools format
func convertResponsesTools(toolsRaw []byte) []dto.ToolCallRequest {
	var toolsMap []map[string]any
	if err := common.Unmarshal(toolsRaw, &toolsMap); err != nil {
		return nil
	}

	tools := make([]dto.ToolCallRequest, 0, len(toolsMap))
	for _, tool := range toolsMap {
		toolType, _ := tool["type"].(string)

		switch toolType {
		case "function":
			// Responses format: {type: "function", name: "...", description: "...", parameters: {...}}
			// Chat format: {type: "function", function: {name: "...", description: "...", parameters: {...}}}
			name, _ := tool["name"].(string)
			description, _ := tool["description"].(string)
			parameters := tool["parameters"]

			tools = append(tools, dto.ToolCallRequest{
				Type: "function",
				Function: dto.FunctionRequest{
					Name:        name,
					Description: description,
					Parameters:  parameters,
				},
			})
		default:
			// For other tool types (web_search, code_interpreter, etc.), keep as-is
			// These will be handled by the specific channel adaptor
			if toolType != "" {
				tools = append(tools, dto.ToolCallRequest{
					Type: toolType,
				})
			}
		}
	}

	return tools
}

// convertResponsesToolChoice converts Responses API tool_choice to Chat Completions format
func convertResponsesToolChoice(toolChoiceRaw []byte) any {
	// Try string first
	if common.GetJsonType(toolChoiceRaw) == "string" {
		var str string
		if err := common.Unmarshal(toolChoiceRaw, &str); err == nil {
			return str
		}
	}

	// Try object
	var toolChoice map[string]any
	if err := common.Unmarshal(toolChoiceRaw, &toolChoice); err != nil {
		return nil
	}

	toolType, _ := toolChoice["type"].(string)
	if toolType == "function" {
		// Responses format: {type: "function", name: "..."}
		// Chat format: {type: "function", function: {name: "..."}}
		name, _ := toolChoice["name"].(string)
		if name != "" {
			return map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": name,
				},
			}
		}
	}

	return toolChoice
}
