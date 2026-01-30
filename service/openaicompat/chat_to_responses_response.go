package openaicompat

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// ChatCompletionsResponseToResponsesResponse converts a Chat Completions response
// to an OpenAI Responses API response format.
//
// Conversion rules:
// - choices[0].message.content → output[{type:"message", content:[{type:"output_text", text:...}]}]
// - choices[0].message.tool_calls → output[{type:"function_call", call_id:..., name:..., arguments:...}]
// - usage.prompt_tokens → usage.input_tokens
// - usage.completion_tokens → usage.output_tokens
func ChatCompletionsResponseToResponsesResponse(
	chatResp *dto.OpenAITextResponse,
	originalReq *dto.OpenAIResponsesRequest,
) *dto.OpenAIResponsesResponse {
	if chatResp == nil {
		return nil
	}

	// Generate response ID
	responseID := chatResp.Id
	if responseID == "" || !strings.HasPrefix(responseID, "resp_") {
		responseID = fmt.Sprintf("resp_%s", common.GetUUID())
	}

	// Get created timestamp
	createdAt := 0
	switch v := chatResp.Created.(type) {
	case int:
		createdAt = v
	case int64:
		createdAt = int(v)
	case float64:
		createdAt = int(v)
	default:
		createdAt = int(time.Now().Unix())
	}

	// Build output array
	output := make([]dto.ResponsesOutput, 0)

	if len(chatResp.Choices) > 0 {
		choice := chatResp.Choices[0]
		msg := choice.Message

		// Check for tool calls first
		toolCalls := msg.ParseToolCalls()
		if len(toolCalls) > 0 {
			// Add function_call outputs for each tool call
			for _, tc := range toolCalls {
				output = append(output, dto.ResponsesOutput{
					Type:      "function_call",
					ID:        fmt.Sprintf("fc_%s", common.GetUUID()),
					Status:    "completed",
					CallId:    tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}
		}

		// Check for text content
		textContent := msg.StringContent()
		if textContent != "" || len(toolCalls) == 0 {
			// Build content array
			contentItems := make([]dto.ResponsesOutputContent, 0)

			// Add reasoning content if present
			if msg.ReasoningContent != "" {
				contentItems = append(contentItems, dto.ResponsesOutputContent{
					Type: "reasoning",
					Text: msg.ReasoningContent,
				})
			}

			// Add text content
			if textContent != "" {
				contentItems = append(contentItems, dto.ResponsesOutputContent{
					Type:        "output_text",
					Text:        textContent,
					Annotations: []interface{}{},
				})
			}

			if len(contentItems) > 0 || len(toolCalls) == 0 {
				output = append([]dto.ResponsesOutput{{
					Type:    "message",
					ID:      fmt.Sprintf("msg_%s", common.GetUUID()),
					Status:  "completed",
					Role:    "assistant",
					Content: contentItems,
				}}, output...)
			}
		}
	}

	// Determine status
	status := "completed"
	if len(chatResp.Choices) > 0 {
		switch chatResp.Choices[0].FinishReason {
		case "length":
			status = "incomplete"
		case "content_filter":
			status = "failed"
		}
	}

	// Convert usage
	usage := convertChatUsageToResponsesUsage(&chatResp.Usage)

	// Get instructions from original request
	instructions := ""
	if originalReq != nil && len(originalReq.Instructions) > 0 {
		_ = common.Unmarshal(originalReq.Instructions, &instructions)
	}

	// Get max_output_tokens from original request
	maxOutputTokens := 0
	if originalReq != nil {
		maxOutputTokens = int(originalReq.MaxOutputTokens)
	}

	// Get temperature
	temperature := float64(0)
	if originalReq != nil && originalReq.Temperature != nil {
		temperature = *originalReq.Temperature
	}

	// Get top_p
	topP := float64(0)
	if originalReq != nil && originalReq.TopP != nil {
		topP = *originalReq.TopP
	}

	// Get reasoning
	var reasoning *dto.Reasoning
	if originalReq != nil && originalReq.Reasoning != nil {
		reasoning = originalReq.Reasoning
	}

	// Get metadata
	var metadata []byte
	if originalReq != nil && len(originalReq.Metadata) > 0 {
		metadata = originalReq.Metadata
	}

	return &dto.OpenAIResponsesResponse{
		ID:              responseID,
		Object:          "response",
		CreatedAt:       createdAt,
		Status:          status,
		Model:           chatResp.Model,
		Output:          output,
		Usage:           usage,
		Instructions:    instructions,
		MaxOutputTokens: maxOutputTokens,
		Temperature:     temperature,
		TopP:            topP,
		Reasoning:       reasoning,
		Metadata:        metadata,
	}
}

// convertChatUsageToResponsesUsage converts Chat Completions usage to Responses API usage format
func convertChatUsageToResponsesUsage(chatUsage *dto.Usage) *dto.Usage {
	if chatUsage == nil {
		return nil
	}

	usage := &dto.Usage{
		PromptTokens:           chatUsage.PromptTokens,
		CompletionTokens:       chatUsage.CompletionTokens,
		TotalTokens:            chatUsage.TotalTokens,
		InputTokens:            chatUsage.PromptTokens,
		OutputTokens:           chatUsage.CompletionTokens,
		PromptTokensDetails:    chatUsage.PromptTokensDetails,
		CompletionTokenDetails: chatUsage.CompletionTokenDetails,
	}

	// Use InputTokens if already set
	if chatUsage.InputTokens > 0 {
		usage.InputTokens = chatUsage.InputTokens
	}
	if chatUsage.OutputTokens > 0 {
		usage.OutputTokens = chatUsage.OutputTokens
	}

	return usage
}

// ResponsesOutputTypeMessage is the type for message outputs
const ResponsesOutputTypeMessage = "message"

// ResponsesOutputTypeFunctionCall is the type for function call outputs
const ResponsesOutputTypeFunctionCall = "function_call"

const ResponsesOutputTypeImageGenerationCall = "image_generation_call"
