package openaicompat

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// ChatToResponsesStreamAdapter handles the conversion of Chat Completions stream chunks
// to OpenAI Responses API stream events.
type ChatToResponsesStreamAdapter struct {
	ResponseID      string
	CreatedAt       int
	Model           string
	OriginalRequest *dto.OpenAIResponsesRequest

	// State tracking
	initialized       bool
	messageItemID     string
	contentPartIndex  int
	toolCallItemIDs   map[int]string // Index -> Item ID
	toolCallArguments map[int]string // Index -> Accumulated arguments
	outputIndex       int
	hasTextContent    bool
	textContentIndex  int

	// Reasoning content tracking
	hasReasoningContent   bool
	reasoningContentIndex int
}

// NewChatToResponsesStreamAdapter creates a new stream adapter
func NewChatToResponsesStreamAdapter(originalReq *dto.OpenAIResponsesRequest) *ChatToResponsesStreamAdapter {
	return &ChatToResponsesStreamAdapter{
		ResponseID:        fmt.Sprintf("resp_%s", common.GetUUID()),
		CreatedAt:         int(common.GetTimestamp()),
		OriginalRequest:   originalReq,
		messageItemID:     fmt.Sprintf("msg_%s", common.GetUUID()),
		toolCallItemIDs:   make(map[int]string),
		toolCallArguments: make(map[int]string),
	}
}

// ConvertChunk converts a Chat Completions stream chunk to Responses stream events.
// Returns a slice of JSON-encoded event strings (without "data: " prefix).
func (a *ChatToResponsesStreamAdapter) ConvertChunk(chunk *dto.ChatCompletionsStreamResponse) [][]byte {
	if chunk == nil {
		return nil
	}

	events := make([][]byte, 0)

	// Update model if present
	if chunk.Model != "" {
		a.Model = chunk.Model
	}

	// Handle initial response.created event
	if !a.initialized {
		a.initialized = true
		events = append(events, a.createResponseCreatedEvent())
		events = append(events, a.createResponseInProgressEvent())
	}

	// Process choices
	if len(chunk.Choices) > 0 {
		choice := chunk.Choices[0]
		delta := choice.Delta

		// Handle role (indicates start of new message)
		if delta.Role == "assistant" && !a.hasTextContent && !a.hasReasoningContent {
			events = append(events, a.createOutputItemAddedEvent())
		}

		// Handle reasoning content first (reasoning comes before text in output)
		if reasoning := delta.GetReasoningContent(); reasoning != "" {
			if !a.hasReasoningContent {
				a.hasReasoningContent = true
				a.reasoningContentIndex = a.contentPartIndex
				a.contentPartIndex++
				events = append(events, a.createReasoningContentPartAddedEvent())
			}
			events = append(events, a.createReasoningDeltaEvent(reasoning))
		}

		// Handle text content delta
		if delta.Content != nil && *delta.Content != "" {
			if !a.hasTextContent {
				a.hasTextContent = true
				a.textContentIndex = a.contentPartIndex
				a.contentPartIndex++
				events = append(events, a.createContentPartAddedEvent())
			}
			events = append(events, a.createTextDeltaEvent(*delta.Content))
		}

		// Handle tool calls
		if len(delta.ToolCalls) > 0 {
			for _, tc := range delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}

				// Check if this is a new tool call
				if _, exists := a.toolCallItemIDs[idx]; !exists {
					// New tool call
					itemID := fmt.Sprintf("fc_%s", common.GetUUID())
					a.toolCallItemIDs[idx] = itemID
					a.toolCallArguments[idx] = ""
					a.outputIndex++

					// Emit output_item.added for function call
					events = append(events, a.createFunctionCallAddedEvent(idx, tc.ID, tc.Function.Name))
				}

				// Handle arguments delta
				if tc.Function.Arguments != "" {
					a.toolCallArguments[idx] += tc.Function.Arguments
					events = append(events, a.createFunctionCallArgumentsDeltaEvent(idx, tc.Function.Arguments))
				}
			}
		}

		// Handle finish reason
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			// Complete reasoning content first (reasoning comes before text in output)
			if a.hasReasoningContent {
				events = append(events, a.createReasoningDoneEvent())
				events = append(events, a.createReasoningContentPartDoneEvent())
			}

			// Complete any pending text content
			if a.hasTextContent {
				events = append(events, a.createTextDoneEvent())
				events = append(events, a.createContentPartDoneEvent())
			}

			// Complete message output item if we have any content
			if a.hasTextContent || a.hasReasoningContent {
				events = append(events, a.createOutputItemDoneEvent())
			}

			// Complete tool calls
			for idx := range a.toolCallItemIDs {
				events = append(events, a.createFunctionCallArgumentsDoneEvent(idx))
				events = append(events, a.createFunctionCallDoneEvent(idx))
			}

			// Create completed response
			events = append(events, a.createResponseCompletedEvent(chunk.Usage, *choice.FinishReason))
		}
	}

	return events
}

// createResponseCreatedEvent creates the response.created event
func (a *ChatToResponsesStreamAdapter) createResponseCreatedEvent() []byte {
	event := map[string]any{
		"type": "response.created",
		"response": map[string]any{
			"id":         a.ResponseID,
			"object":     "response",
			"created_at": a.CreatedAt,
			"status":     "in_progress",
			"model":      a.Model,
			"output":     []any{},
		},
	}
	data, _ := common.Marshal(event)
	return data
}

// createResponseInProgressEvent creates the response.in_progress event
func (a *ChatToResponsesStreamAdapter) createResponseInProgressEvent() []byte {
	event := map[string]any{
		"type": "response.in_progress",
		"response": map[string]any{
			"id":         a.ResponseID,
			"object":     "response",
			"created_at": a.CreatedAt,
			"status":     "in_progress",
			"model":      a.Model,
		},
	}
	data, _ := common.Marshal(event)
	return data
}

// createOutputItemAddedEvent creates the response.output_item.added event for message
func (a *ChatToResponsesStreamAdapter) createOutputItemAddedEvent() []byte {
	event := map[string]any{
		"type":         "response.output_item.added",
		"output_index": a.outputIndex,
		"item": map[string]any{
			"type":    "message",
			"id":      a.messageItemID,
			"status":  "in_progress",
			"role":    "assistant",
			"content": []any{},
		},
	}
	data, _ := common.Marshal(event)
	return data
}

// createContentPartAddedEvent creates the response.content_part.added event
func (a *ChatToResponsesStreamAdapter) createContentPartAddedEvent() []byte {
	event := map[string]any{
		"type":          "response.content_part.added",
		"item_id":       a.messageItemID,
		"output_index":  a.outputIndex,
		"content_index": a.textContentIndex,
		"part": map[string]any{
			"type": "output_text",
			"text": "",
		},
	}
	data, _ := common.Marshal(event)
	return data
}

// createTextDeltaEvent creates the response.output_text.delta event
func (a *ChatToResponsesStreamAdapter) createTextDeltaEvent(text string) []byte {
	event := map[string]any{
		"type":          "response.output_text.delta",
		"item_id":       a.messageItemID,
		"output_index":  a.outputIndex,
		"content_index": a.textContentIndex,
		"delta":         text,
	}
	data, _ := common.Marshal(event)
	return data
}

// createReasoningContentPartAddedEvent creates the response.content_part.added event for reasoning
func (a *ChatToResponsesStreamAdapter) createReasoningContentPartAddedEvent() []byte {
	event := map[string]any{
		"type":          "response.content_part.added",
		"item_id":       a.messageItemID,
		"output_index":  a.outputIndex,
		"content_index": a.reasoningContentIndex,
		"part": map[string]any{
			"type": "reasoning",
			"text": "",
		},
	}
	data, _ := common.Marshal(event)
	return data
}

// createReasoningDeltaEvent creates the response.reasoning.delta event
func (a *ChatToResponsesStreamAdapter) createReasoningDeltaEvent(text string) []byte {
	event := map[string]any{
		"type":          "response.reasoning.delta",
		"item_id":       a.messageItemID,
		"output_index":  a.outputIndex,
		"content_index": a.reasoningContentIndex,
		"delta":         text,
	}
	data, _ := common.Marshal(event)
	return data
}

// createReasoningDoneEvent creates the response.reasoning.done event
func (a *ChatToResponsesStreamAdapter) createReasoningDoneEvent() []byte {
	event := map[string]any{
		"type":          "response.reasoning.done",
		"item_id":       a.messageItemID,
		"output_index":  a.outputIndex,
		"content_index": a.reasoningContentIndex,
		"text":          "",
	}
	data, _ := common.Marshal(event)
	return data
}

// createReasoningContentPartDoneEvent creates the response.content_part.done event for reasoning
func (a *ChatToResponsesStreamAdapter) createReasoningContentPartDoneEvent() []byte {
	event := map[string]any{
		"type":          "response.content_part.done",
		"item_id":       a.messageItemID,
		"output_index":  a.outputIndex,
		"content_index": a.reasoningContentIndex,
		"part": map[string]any{
			"type": "reasoning",
			"text": "",
		},
	}
	data, _ := common.Marshal(event)
	return data
}

// createTextDoneEvent creates the response.output_text.done event
func (a *ChatToResponsesStreamAdapter) createTextDoneEvent() []byte {
	event := map[string]any{
		"type":          "response.output_text.done",
		"item_id":       a.messageItemID,
		"output_index":  a.outputIndex,
		"content_index": a.textContentIndex,
		"text":          "", // Full text would be accumulated, but we don't track it
	}
	data, _ := common.Marshal(event)
	return data
}

// createContentPartDoneEvent creates the response.content_part.done event
func (a *ChatToResponsesStreamAdapter) createContentPartDoneEvent() []byte {
	event := map[string]any{
		"type":          "response.content_part.done",
		"item_id":       a.messageItemID,
		"output_index":  a.outputIndex,
		"content_index": a.textContentIndex,
		"part": map[string]any{
			"type": "output_text",
			"text": "",
		},
	}
	data, _ := common.Marshal(event)
	return data
}

// createOutputItemDoneEvent creates the response.output_item.done event for message
func (a *ChatToResponsesStreamAdapter) createOutputItemDoneEvent() []byte {
	content := a.buildMessageContent(false)

	event := map[string]any{
		"type":         "response.output_item.done",
		"output_index": a.outputIndex,
		"item": map[string]any{
			"type":    "message",
			"id":      a.messageItemID,
			"status":  "completed",
			"role":    "assistant",
			"content": content,
		},
	}
	data, _ := common.Marshal(event)
	return data
}

// createFunctionCallAddedEvent creates the response.output_item.added event for function call
func (a *ChatToResponsesStreamAdapter) createFunctionCallAddedEvent(idx int, callID, name string) []byte {
	event := map[string]any{
		"type":         "response.output_item.added",
		"output_index": a.outputIndex,
		"item": map[string]any{
			"type":      "function_call",
			"id":        a.toolCallItemIDs[idx],
			"status":    "in_progress",
			"call_id":   callID,
			"name":      name,
			"arguments": "",
		},
	}
	data, _ := common.Marshal(event)
	return data
}

// createFunctionCallArgumentsDeltaEvent creates the response.function_call_arguments.delta event
func (a *ChatToResponsesStreamAdapter) createFunctionCallArgumentsDeltaEvent(idx int, argsDelta string) []byte {
	outputIdx := a.outputIndex
	if a.hasTextContent || a.hasReasoningContent {
		outputIdx = idx + 1 // Adjust for message output
	} else {
		outputIdx = idx
	}

	event := map[string]any{
		"type":         "response.function_call_arguments.delta",
		"item_id":      a.toolCallItemIDs[idx],
		"output_index": outputIdx,
		"delta":        argsDelta,
	}
	data, _ := common.Marshal(event)
	return data
}

// createFunctionCallArgumentsDoneEvent creates the response.function_call_arguments.done event
func (a *ChatToResponsesStreamAdapter) createFunctionCallArgumentsDoneEvent(idx int) []byte {
	outputIdx := idx
	if a.hasTextContent || a.hasReasoningContent {
		outputIdx = idx + 1
	}

	event := map[string]any{
		"type":         "response.function_call_arguments.done",
		"item_id":      a.toolCallItemIDs[idx],
		"output_index": outputIdx,
		"arguments":    a.toolCallArguments[idx],
	}
	data, _ := common.Marshal(event)
	return data
}

// createFunctionCallDoneEvent creates the response.output_item.done event for function call
func (a *ChatToResponsesStreamAdapter) createFunctionCallDoneEvent(idx int) []byte {
	outputIdx := idx
	if a.hasTextContent || a.hasReasoningContent {
		outputIdx = idx + 1
	}

	event := map[string]any{
		"type":         "response.output_item.done",
		"output_index": outputIdx,
		"item": map[string]any{
			"type":      "function_call",
			"id":        a.toolCallItemIDs[idx],
			"status":    "completed",
			"arguments": a.toolCallArguments[idx],
		},
	}
	data, _ := common.Marshal(event)
	return data
}

// createResponseCompletedEvent creates the response.completed event
func (a *ChatToResponsesStreamAdapter) createResponseCompletedEvent(usage *dto.Usage, finishReason string) []byte {
	status := "completed"
	switch finishReason {
	case "length":
		status = "incomplete"
	case "content_filter":
		status = "failed"
	}

	// Build output array
	output := make([]map[string]any, 0)

	if a.hasTextContent || a.hasReasoningContent {
		content := a.buildMessageContent(true)

		output = append(output, map[string]any{
			"type":    "message",
			"id":      a.messageItemID,
			"status":  "completed",
			"role":    "assistant",
			"content": content,
		})
	}

	for idx, itemID := range a.toolCallItemIDs {
		output = append(output, map[string]any{
			"type":      "function_call",
			"id":        itemID,
			"status":    "completed",
			"arguments": a.toolCallArguments[idx],
		})
	}

	// Convert usage
	var usageMap map[string]any
	if usage != nil {
		usageMap = map[string]any{
			"input_tokens":  usage.PromptTokens,
			"output_tokens": usage.CompletionTokens,
			"total_tokens":  usage.TotalTokens,
		}
		if usage.InputTokens > 0 {
			usageMap["input_tokens"] = usage.InputTokens
		}
		if usage.OutputTokens > 0 {
			usageMap["output_tokens"] = usage.OutputTokens
		}
	}

	event := map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"id":         a.ResponseID,
			"object":     "response",
			"created_at": a.CreatedAt,
			"status":     status,
			"model":      a.Model,
			"output":     output,
			"usage":      usageMap,
		},
	}
	data, _ := common.Marshal(event)
	return data
}

// GetResponseID returns the response ID
func (a *ChatToResponsesStreamAdapter) GetResponseID() string {
	return a.ResponseID
}

func (a *ChatToResponsesStreamAdapter) buildMessageContent(withAnnotations bool) []map[string]any {
	parts := make([]map[string]any, 0, 2)
	if !a.hasReasoningContent && !a.hasTextContent {
		return parts
	}

	addReasoning := func() {
		parts = append(parts, map[string]any{
			"type": "reasoning",
			"text": "",
		})
	}
	addText := func() {
		part := map[string]any{
			"type": "output_text",
			"text": "",
		}
		if withAnnotations {
			part["annotations"] = []any{}
		}
		parts = append(parts, part)
	}

	if a.hasReasoningContent && a.hasTextContent {
		if a.reasoningContentIndex <= a.textContentIndex {
			addReasoning()
			addText()
		} else {
			addText()
			addReasoning()
		}
		return parts
	}

	if a.hasReasoningContent {
		addReasoning()
		return parts
	}

	addText()
	return parts
}
