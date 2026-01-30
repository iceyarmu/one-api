package service

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/openaicompat"
)

func ChatCompletionsRequestToResponsesRequest(req *dto.GeneralOpenAIRequest) (*dto.OpenAIResponsesRequest, error) {
	return openaicompat.ChatCompletionsRequestToResponsesRequest(req)
}

func ResponsesResponseToChatCompletionsResponse(resp *dto.OpenAIResponsesResponse, id string) (*dto.OpenAITextResponse, *dto.Usage, error) {
	return openaicompat.ResponsesResponseToChatCompletionsResponse(resp, id)
}

func ExtractOutputTextFromResponses(resp *dto.OpenAIResponsesResponse) string {
	return openaicompat.ExtractOutputTextFromResponses(resp)
}

// ResponsesRequestToChatCompletionsRequest converts an OpenAI Responses API request
// to a Chat Completions API request for channels that don't support Responses API natively.
func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	return openaicompat.ResponsesRequestToChatCompletionsRequest(req)
}

// ChatCompletionsResponseToResponsesResponse converts a Chat Completions response
// to an OpenAI Responses API response format.
func ChatCompletionsResponseToResponsesResponse(chatResp *dto.OpenAITextResponse, originalReq *dto.OpenAIResponsesRequest) *dto.OpenAIResponsesResponse {
	return openaicompat.ChatCompletionsResponseToResponsesResponse(chatResp, originalReq)
}

// NewChatToResponsesStreamAdapter creates a new stream adapter for converting
// Chat Completions stream to Responses stream format.
func NewChatToResponsesStreamAdapter(originalReq *dto.OpenAIResponsesRequest) *openaicompat.ChatToResponsesStreamAdapter {
	return openaicompat.NewChatToResponsesStreamAdapter(originalReq)
}
