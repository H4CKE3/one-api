package gemini

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/songquanpeng/one-api/common/render"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/image"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/common/random"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/constant"
	"github.com/songquanpeng/one-api/relay/model"

	"github.com/gin-gonic/gin"
)

// https://ai.google.dev/docs/gemini_api_overview?hl=zh-cn

const (
	VisionMaxImageNum = 16
)

var mimeTypeMap = map[string]string{
	"json_object": "application/json",
	"text":        "text/plain",
}

// Setting safety to the lowest possible values since Gemini is already powerless enough
func ConvertRequest(textRequest model.GeneralOpenAIRequest) *ChatRequest {
	geminiRequest := ChatRequest{
		Contents: make([]ChatContent, 0, len(textRequest.Messages)),
		SafetySettings: []ChatSafetySettings{
			{
				Category:  "HARM_CATEGORY_HARASSMENT",
				Threshold: config.GeminiSafetySetting,
			},
			{
				Category:  "HARM_CATEGORY_HATE_SPEECH",
				Threshold: config.GeminiSafetySetting,
			},
			{
				Category:  "HARM_CATEGORY_SEXUALLY_EXPLICIT",
				Threshold: config.GeminiSafetySetting,
			},
			{
				Category:  "HARM_CATEGORY_DANGEROUS_CONTENT",
				Threshold: config.GeminiSafetySetting,
			},
			{
				Category:  "HARM_CATEGORY_CIVIC_INTEGRITY",
				Threshold: config.GeminiSafetySetting,
			},
		},
		GenerationConfig: ChatGenerationConfig{
			Temperature:     textRequest.Temperature,
			TopP:            textRequest.TopP,
			MaxOutputTokens: textRequest.MaxTokens,
		},
	}
	if textRequest.ResponseFormat != nil {
		if mimeType, ok := mimeTypeMap[textRequest.ResponseFormat.Type]; ok {
			geminiRequest.GenerationConfig.ResponseMimeType = mimeType
		}
		if textRequest.ResponseFormat.JsonSchema != nil {
			geminiRequest.GenerationConfig.ResponseSchema = textRequest.ResponseFormat.JsonSchema.Schema
			geminiRequest.GenerationConfig.ResponseMimeType = mimeTypeMap["json_object"]
		}
	}
	// 处理 thinking_budget 配置
	if textRequest.GenerationConfig != nil && textRequest.GenerationConfig.ThinkingConfig != nil && textRequest.GenerationConfig.ThinkingConfig.ThinkingBudget > 0 {
		geminiRequest.GenerationConfig.ThinkingConfig = &ThinkingConfig{
			ThinkingBudget: textRequest.GenerationConfig.ThinkingConfig.ThinkingBudget,
		}
	}
	if textRequest.Tools != nil {
		functions := make([]model.Function, 0, len(textRequest.Tools))
		tools := make([]ChatTools, 0, len(textRequest.Tools))
		for _, tool := range textRequest.Tools {
			switch {
			case tool.Type == "google_search" || tool.GoogleSearch != nil:
				tools = append(tools, ChatTools{GoogleSearch: assignOrEmptyToolConfig(tool.GoogleSearch)})
			case tool.Type == "google_search_retrieval" || tool.GoogleSearchRetrieval != nil:
				tools = append(tools, ChatTools{GoogleSearchRetrieval: assignOrEmptyToolConfig(tool.GoogleSearchRetrieval)})
			case tool.Function.Name != "":
				functions = append(functions, tool.Function)
			}
		}
		if len(functions) > 0 {
			tools = append(tools, ChatTools{
				FunctionDeclarations: functions,
			})
		}
		if len(tools) > 0 {
			geminiRequest.Tools = tools
		}
	} else if textRequest.Functions != nil {
		geminiRequest.Tools = []ChatTools{
			{
				FunctionDeclarations: textRequest.Functions,
			},
		}
	}
	shouldAddDummyModelMessage := false
	toolCallNames := make(map[string]string)
	for _, message := range textRequest.Messages {
		content := ChatContent{
			Role: message.Role,
			Parts: []Part{
				{
					Text: message.StringContent(),
				},
			},
		}
		openaiContent := message.ParseContent()
		var parts []Part
		imageNum := 0
		for _, part := range openaiContent {
			if part.Type == model.ContentTypeText {
				parts = append(parts, Part{
					Text: part.Text,
				})
			} else if part.Type == model.ContentTypeImageURL {
				imageNum += 1
				if imageNum > VisionMaxImageNum {
					continue
				}
				mimeType, data, _ := image.GetImageFromUrl(part.ImageURL.Url)
				parts = append(parts, Part{
					InlineData: &InlineData{
						MimeType: mimeType,
						Data:     data,
					},
				})
			} else if part.Type == model.ContentTypeFile && part.File != nil && part.File.FileData != "" {
				mimeType, data, err := model.ParseFileDataURI(part.File.FileData)
				if err != nil {
					logger.SysError("error parsing file data: " + err.Error())
					continue
				}
				parts = append(parts, Part{
					InlineData: &InlineData{
						MimeType: mimeType,
						Data:     data,
					},
				})
			}
		}
		content.Parts = parts

		if len(message.ToolCalls) > 0 {
			content.Parts = openAIToolCallsToGeminiParts(message.ToolCalls, toolCallNames)
		}
		if message.Role == "tool" {
			content.Role = "user"
			content.Parts = []Part{
				openAIToolResultToGeminiPart(message, toolCallNames),
			}
		}
		// there's no assistant role in gemini and API shall vomit if Role is not user or model
		if content.Role == "assistant" {
			content.Role = "model"
		}
		// Converting system prompt to prompt from user for the same reason
		if content.Role == "system" {
			shouldAddDummyModelMessage = true
			if IsModelSupportSystemInstruction(textRequest.Model) {
				geminiRequest.SystemInstruction = &content
				geminiRequest.SystemInstruction.Role = ""
				continue
			} else {
				content.Role = "user"
			}
		}

		geminiRequest.Contents = append(geminiRequest.Contents, content)

		// If a system message is the last message, we need to add a dummy model message to make gemini happy
		if shouldAddDummyModelMessage {
			geminiRequest.Contents = append(geminiRequest.Contents, ChatContent{
				Role: "model",
				Parts: []Part{
					{
						Text: "Okay",
					},
				},
			})
			shouldAddDummyModelMessage = false
		}
	}

	return &geminiRequest
}

func openAIToolCallsToGeminiParts(toolCalls []model.Tool, toolCallNames map[string]string) []Part {
	parts := make([]Part, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		functionName := toolCall.Function.Name
		if toolCall.Id != "" && functionName != "" {
			toolCallNames[toolCall.Id] = functionName
		}
		if functionName == "" {
			continue
		}
		parts = append(parts, Part{
			FunctionCall: &FunctionCall{
				FunctionName: functionName,
				Arguments:    parseFunctionArguments(toolCall.Function.Arguments),
			},
		})
	}
	return parts
}

func openAIToolResultToGeminiPart(message model.Message, toolCallNames map[string]string) Part {
	functionName := ""
	if message.Name != nil {
		functionName = *message.Name
	}
	if functionName == "" {
		functionName = toolCallNames[message.ToolCallId]
	}
	if functionName == "" {
		functionName = message.ToolCallId
	}
	return Part{
		FunctionResponse: &FunctionResponse{
			FunctionName: functionName,
			Response:     parseFunctionResponse(message.Content),
		},
	}
}

func parseFunctionArguments(arguments any) any {
	if value, ok := arguments.(string); ok {
		var parsed any
		if err := json.Unmarshal([]byte(value), &parsed); err == nil {
			return parsed
		}
	}
	return arguments
}

func parseFunctionResponse(content any) any {
	if value, ok := content.(string); ok {
		var parsed any
		if err := json.Unmarshal([]byte(value), &parsed); err == nil {
			if _, ok := parsed.(map[string]any); ok {
				return parsed
			}
		}
		return map[string]any{"result": value}
	}
	return map[string]any{"result": content}
}

func assignOrEmptyToolConfig(value map[string]any) any {
	if value == nil {
		return struct{}{}
	}
	return value
}

func ConvertEmbeddingRequest(request model.GeneralOpenAIRequest) *BatchEmbeddingRequest {
	inputs := request.ParseInput()
	requests := make([]EmbeddingRequest, len(inputs))
	model := fmt.Sprintf("models/%s", request.Model)

	for i, input := range inputs {
		requests[i] = EmbeddingRequest{
			Model: model,
			Content: ChatContent{
				Parts: []Part{
					{
						Text: input,
					},
				},
			},
		}
	}

	return &BatchEmbeddingRequest{
		Requests: requests,
	}
}

type ChatResponse struct {
	Candidates     []ChatCandidate    `json:"candidates"`
	PromptFeedback ChatPromptFeedback `json:"promptFeedback"`
	UsageMetadata  UsageMetadata      `json:"usageMetadata"`
}

func (g *ChatResponse) GetResponseText() string {
	if g == nil {
		return ""
	}
	if len(g.Candidates) == 0 {
		return ""
	}
	return getCandidateText(&g.Candidates[0])
}

type UsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	TotalTokenCount         int `json:"totalTokenCount"`
	ToolUsePromptTokenCount int `json:"toolUsePromptTokenCount"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount"`
}

func (u UsageMetadata) IsZero() bool {
	return u.PromptTokenCount == 0 &&
		u.CandidatesTokenCount == 0 &&
		u.TotalTokenCount == 0 &&
		u.ToolUsePromptTokenCount == 0 &&
		u.ThoughtsTokenCount == 0
}

func (u UsageMetadata) ToOpenAIUsage() *model.Usage {
	promptTokens := u.PromptTokenCount + u.ToolUsePromptTokenCount
	completionTokens := u.CandidatesTokenCount + u.ThoughtsTokenCount
	totalTokens := u.TotalTokenCount
	if totalTokens == 0 {
		totalTokens = promptTokens + completionTokens
	}
	if totalTokens > 0 && promptTokens+completionTokens != totalTokens {
		completionTokens = totalTokens - promptTokens
	}

	usage := &model.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
	}
	if u.ThoughtsTokenCount > 0 {
		usage.CompletionTokensDetails = &model.CompletionTokensDetails{
			ReasoningTokens: u.ThoughtsTokenCount,
		}
	}
	return usage
}

type ChatCandidate struct {
	Content       ChatContent        `json:"content"`
	FinishReason  string             `json:"finishReason"`
	Index         int64              `json:"index"`
	SafetyRatings []ChatSafetyRating `json:"safetyRatings"`
}

type ChatSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

type ChatPromptFeedback struct {
	SafetyRatings []ChatSafetyRating `json:"safetyRatings"`
}

func getToolCalls(candidate *ChatCandidate) []model.Tool {
	var toolCalls []model.Tool

	item := candidate.Content.Parts[0]
	if item.FunctionCall == nil {
		return toolCalls
	}
	argsBytes, err := json.Marshal(item.FunctionCall.Arguments)
	if err != nil {
		logger.FatalLog("getToolCalls failed: " + err.Error())
		return toolCalls
	}
	toolCall := model.Tool{
		Id:   fmt.Sprintf("call_%s", random.GetUUID()),
		Type: "function",
		Function: model.Function{
			Arguments: string(argsBytes),
			Name:      item.FunctionCall.FunctionName,
		},
	}
	toolCalls = append(toolCalls, toolCall)
	return toolCalls
}

func getCandidateText(candidate *ChatCandidate) string {
	if candidate == nil {
		return ""
	}
	var builder strings.Builder
	for _, part := range candidate.Content.Parts {
		builder.WriteString(part.Text)
	}
	return builder.String()
}

func hasToolCall(candidate *ChatCandidate) bool {
	if candidate == nil {
		return false
	}
	for _, part := range candidate.Content.Parts {
		if part.FunctionCall != nil {
			return true
		}
	}
	return false
}

func emptyVisibleContentDiagnostic(response *ChatResponse, usage *model.Usage) string {
	if response == nil || len(response.Candidates) == 0 || hasToolCall(&response.Candidates[0]) {
		return ""
	}
	candidate := response.Candidates[0]
	if strings.TrimSpace(getCandidateText(&candidate)) != "" {
		return ""
	}

	parts := []string{"Gemini returned no visible content"}
	if candidate.FinishReason != "" {
		parts = append(parts, "finishReason="+candidate.FinishReason)
	}
	if usage != nil {
		parts = append(parts,
			"promptTokens="+strconv.Itoa(usage.PromptTokens),
			"completionTokens="+strconv.Itoa(usage.CompletionTokens),
			"totalTokens="+strconv.Itoa(usage.TotalTokens),
		)
	}
	if len(candidate.SafetyRatings) > 0 {
		ratings := make([]string, 0, len(candidate.SafetyRatings))
		for _, rating := range candidate.SafetyRatings {
			if rating.Category == "" && rating.Probability == "" {
				continue
			}
			ratings = append(ratings, rating.Category+":"+rating.Probability)
		}
		if len(ratings) > 0 {
			parts = append(parts, "safetyRatings="+strings.Join(ratings, ","))
		}
	}
	return strings.Join(parts, "; ")
}

func responseGeminiChat2OpenAI(response *ChatResponse) *openai.TextResponse {
	fullTextResponse := openai.TextResponse{
		Id:      fmt.Sprintf("chatcmpl-%s", random.GetUUID()),
		Object:  "chat.completion",
		Created: helper.GetTimestamp(),
		Choices: make([]openai.TextResponseChoice, 0, len(response.Candidates)),
	}
	for i, candidate := range response.Candidates {
		choice := openai.TextResponseChoice{
			Index: i,
			Message: model.Message{
				Role: "assistant",
			},
			FinishReason: constant.StopFinishReason,
		}
		if len(candidate.Content.Parts) > 0 {
			if candidate.Content.Parts[0].FunctionCall != nil {
				choice.Message.ToolCalls = getToolCalls(&candidate)
			} else {
				choice.Message.Content = getCandidateText(&candidate)
			}
		} else {
			choice.Message.Content = ""
			choice.FinishReason = candidate.FinishReason
		}
		fullTextResponse.Choices = append(fullTextResponse.Choices, choice)
	}
	return &fullTextResponse
}

func streamResponseGeminiChat2OpenAI(geminiResponse *ChatResponse) *openai.ChatCompletionsStreamResponse {
	var choice openai.ChatCompletionsStreamResponseChoice
	choice.Delta.Content = geminiResponse.GetResponseText()
	//choice.FinishReason = &constant.StopFinishReason
	var response openai.ChatCompletionsStreamResponse
	response.Id = fmt.Sprintf("chatcmpl-%s", random.GetUUID())
	response.Created = helper.GetTimestamp()
	response.Object = "chat.completion.chunk"
	response.Model = "gemini"
	response.Choices = []openai.ChatCompletionsStreamResponseChoice{choice}
	return &response
}

func embeddingResponseGemini2OpenAI(response *EmbeddingResponse) *openai.EmbeddingResponse {
	openAIEmbeddingResponse := openai.EmbeddingResponse{
		Object: "list",
		Data:   make([]openai.EmbeddingResponseItem, 0, len(response.Embeddings)),
		Model:  "gemini-embedding",
		Usage:  model.Usage{TotalTokens: 0},
	}
	for _, item := range response.Embeddings {
		openAIEmbeddingResponse.Data = append(openAIEmbeddingResponse.Data, openai.EmbeddingResponseItem{
			Object:    `embedding`,
			Index:     0,
			Embedding: item.Values,
		})
	}
	return &openAIEmbeddingResponse
}

func StreamHandler(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, string) {
	err, responseText, _ := StreamHandlerWithUsage(c, resp)
	return err, responseText
}

func StreamHandlerWithUsage(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, string, *model.Usage) {
	responseText := ""
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)
	var usage *model.Usage

	common.SetEventStreamHeaders(c)

	for scanner.Scan() {
		data := scanner.Text()
		data = strings.TrimSpace(data)
		if !strings.HasPrefix(data, "data: ") {
			continue
		}
		data = strings.TrimPrefix(data, "data: ")
		data = strings.TrimSuffix(data, "\"")

		var geminiResponse ChatResponse
		err := json.Unmarshal([]byte(data), &geminiResponse)
		if err != nil {
			logger.SysError("error unmarshalling stream response: " + err.Error())
			continue
		}
		if !geminiResponse.UsageMetadata.IsZero() {
			usage = geminiResponse.UsageMetadata.ToOpenAIUsage()
		}

		response := streamResponseGeminiChat2OpenAI(&geminiResponse)
		if response == nil {
			continue
		}

		responseText += response.Choices[0].Delta.StringContent()

		err = render.ObjectData(c, response)
		if err != nil {
			logger.SysError(err.Error())
		}
	}

	if err := scanner.Err(); err != nil {
		logger.SysError("error reading stream: " + err.Error())
	}

	render.Done(c)

	err := resp.Body.Close()
	if err != nil {
		return openai.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), "", nil
	}

	return nil, responseText, usage
}

func Handler(c *gin.Context, resp *http.Response, promptTokens int, modelName string) (*model.ErrorWithStatusCode, *model.Usage, string) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError), nil, ""
	}
	err = resp.Body.Close()
	if err != nil {
		return openai.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), nil, ""
	}
	var geminiResponse ChatResponse
	err = json.Unmarshal(responseBody, &geminiResponse)
	if err != nil {
		return openai.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError), nil, ""
	}
	if len(geminiResponse.Candidates) == 0 {
		return &model.ErrorWithStatusCode{
			Error: model.Error{
				Message: "No candidates returned",
				Type:    "server_error",
				Param:   "",
				Code:    500,
			},
			StatusCode: resp.StatusCode,
		}, nil, ""
	}
	fullTextResponse := responseGeminiChat2OpenAI(&geminiResponse)
	fullTextResponse.Model = modelName

	var usage *model.Usage
	if geminiResponse.UsageMetadata.IsZero() {
		responseText := geminiResponse.GetResponseText()
		completionTokens := openai.CountTokenText(responseText, modelName)
		usage = &model.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	} else {
		usage = geminiResponse.UsageMetadata.ToOpenAIUsage()
	}
	// Keep billing based on Gemini usage, but make empty visible responses traceable
	// in chat records and logs.
	responseText := geminiResponse.GetResponseText()
	if responseText == "" {
		responseText = emptyVisibleContentDiagnostic(&geminiResponse, usage)
	}
	fullTextResponse.Usage = *usage
	jsonResponse, err := json.Marshal(fullTextResponse)
	if err != nil {
		return openai.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError), nil, ""
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, err = c.Writer.Write(jsonResponse)
	return nil, usage, responseText
}

func EmbeddingHandler(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, *model.Usage, string) {
	var geminiEmbeddingResponse EmbeddingResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError), nil, ""
	}
	err = resp.Body.Close()
	if err != nil {
		return openai.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), nil, ""
	}
	err = json.Unmarshal(responseBody, &geminiEmbeddingResponse)
	if err != nil {
		return openai.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError), nil, ""
	}
	if geminiEmbeddingResponse.Error != nil {
		return &model.ErrorWithStatusCode{
			Error: model.Error{
				Message: geminiEmbeddingResponse.Error.Message,
				Type:    "gemini_error",
				Param:   "",
				Code:    geminiEmbeddingResponse.Error.Code,
			},
			StatusCode: resp.StatusCode,
		}, nil, ""
	}
	fullTextResponse := embeddingResponseGemini2OpenAI(&geminiEmbeddingResponse)
	jsonResponse, err := json.Marshal(fullTextResponse)
	if err != nil {
		return openai.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError), nil, ""
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, err = c.Writer.Write(jsonResponse)
	return nil, &fullTextResponse.Usage, ""
}
