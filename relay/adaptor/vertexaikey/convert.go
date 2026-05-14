package vertexaikey

import (
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/image"
	"github.com/songquanpeng/one-api/relay/adaptor/gemini"
	relaymodel "github.com/songquanpeng/one-api/relay/model"
)

var defaultSafetySettings = []ChatSafetySettings{
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
}

func ConvertRequest(textRequest relaymodel.GeneralOpenAIRequest) *ChatRequest {
	vertexRequest := &ChatRequest{
		Contents:       make([]ChatContent, 0, len(textRequest.Messages)),
		SafetySettings: append([]ChatSafetySettings(nil), defaultSafetySettings...),
		GenerationConfig: ChatGenerationConfig{
			Temperature: textRequest.Temperature,
			TopP:        textRequest.TopP,
		},
	}

	if textRequest.TopK > 0 {
		vertexRequest.GenerationConfig.TopK = textRequest.TopK
	}
	if textRequest.N > 0 {
		vertexRequest.GenerationConfig.CandidateCount = textRequest.N
	}
	if textRequest.MaxCompletionTokens != nil && *textRequest.MaxCompletionTokens > 0 {
		vertexRequest.GenerationConfig.MaxOutputTokens = *textRequest.MaxCompletionTokens
	} else if textRequest.MaxTokens > 0 {
		vertexRequest.GenerationConfig.MaxOutputTokens = textRequest.MaxTokens
	}
	if textRequest.PresencePenalty != nil {
		vertexRequest.GenerationConfig.PresencePenalty = textRequest.PresencePenalty
	}
	if textRequest.FrequencyPenalty != nil {
		vertexRequest.GenerationConfig.FrequencyPenalty = textRequest.FrequencyPenalty
	}
	if stopSequences := parseStopSequences(textRequest.Stop); len(stopSequences) > 0 {
		vertexRequest.GenerationConfig.StopSequences = stopSequences
	}
	if textRequest.Seed != 0 {
		vertexRequest.GenerationConfig.Seed = int(textRequest.Seed)
	}
	if textRequest.GenerationConfig != nil && textRequest.GenerationConfig.ThinkingConfig != nil && textRequest.GenerationConfig.ThinkingConfig.ThinkingBudget > 0 {
		vertexRequest.GenerationConfig.ThinkingConfig = &ThinkingConfig{
			ThinkingBudget: textRequest.GenerationConfig.ThinkingConfig.ThinkingBudget,
		}
	}
	if textRequest.ResponseFormat != nil {
		if mimeType, ok := mimeTypeMap[textRequest.ResponseFormat.Type]; ok {
			vertexRequest.GenerationConfig.ResponseMimeType = mimeType
		}
		if textRequest.ResponseFormat.JsonSchema != nil {
			vertexRequest.GenerationConfig.ResponseSchema = textRequest.ResponseFormat.JsonSchema.Schema
			vertexRequest.GenerationConfig.ResponseMimeType = mimeTypeMap["json_object"]
		}
	}
	if textRequest.Tools != nil {
		functions := make([]relaymodel.Function, 0, len(textRequest.Tools))
		for _, tool := range textRequest.Tools {
			functions = append(functions, tool.Function)
		}
		vertexRequest.Tools = []ChatTools{
			{
				FunctionDeclarations: functions,
			},
		}
	} else if textRequest.Functions != nil {
		vertexRequest.Tools = []ChatTools{
			{
				FunctionDeclarations: textRequest.Functions,
			},
		}
	}
	if textRequest.SystemInstructions != nil && len(textRequest.SystemInstructions.Parts) > 0 {
		vertexRequest.SystemInstruction = systemInstructionFromParts(textRequest.SystemInstructions.Parts)
	}

	shouldAddDummyModelMessage := false
	for _, message := range textRequest.Messages {
		content := ChatContent{
			Role: message.Role,
			Parts: []Part{
				{
					Text: message.StringContent(),
				},
			},
		}

		openAIContent := message.ParseContent()
		parts := make([]Part, 0, len(openAIContent))
		imageNum := 0
		for _, part := range openAIContent {
			switch part.Type {
			case relaymodel.ContentTypeText:
				parts = append(parts, Part{
					Text: part.Text,
				})
			case relaymodel.ContentTypeImageURL:
				imageNum++
				if imageNum > gemini.VisionMaxImageNum {
					continue
				}
				mimeType, data, _ := image.GetImageFromUrl(part.ImageURL.Url)
				parts = append(parts, Part{
					InlineData: &InlineData{
						MimeType: mimeType,
						Data:     data,
					},
				})
			case relaymodel.ContentTypeFile:
				if part.File == nil || part.File.FileData == "" {
					continue
				}
				mimeType, data, err := relaymodel.ParseFileDataURI(part.File.FileData)
				if err != nil {
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
		if len(parts) > 0 {
			content.Parts = parts
		}

		if content.Role == "assistant" {
			content.Role = "model"
		}
		if content.Role == "system" {
			shouldAddDummyModelMessage = true
			if vertexRequest.SystemInstruction == nil && gemini.IsModelSupportSystemInstruction(textRequest.Model) {
				content.Role = ""
				vertexRequest.SystemInstruction = &content
				continue
			}
			content.Role = "user"
		}

		vertexRequest.Contents = append(vertexRequest.Contents, content)
		if shouldAddDummyModelMessage {
			vertexRequest.Contents = append(vertexRequest.Contents, ChatContent{
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

	return vertexRequest
}

func parseStopSequences(stop any) []string {
	switch value := stop.(type) {
	case string:
		if value == "" {
			return nil
		}
		return []string{value}
	case []string:
		result := make([]string, 0, len(value))
		for _, item := range value {
			if item != "" {
				result = append(result, item)
			}
		}
		return result
	case []any:
		result := make([]string, 0, len(value))
		for _, item := range value {
			str, ok := item.(string)
			if ok && str != "" {
				result = append(result, str)
			}
		}
		return result
	default:
		return nil
	}
}

func systemInstructionFromParts(parts []relaymodel.SystemInstructionPart) *ChatContent {
	contentParts := make([]Part, 0, len(parts))
	for _, part := range parts {
		if part.Text == "" {
			continue
		}
		contentParts = append(contentParts, Part{Text: part.Text})
	}
	if len(contentParts) == 0 {
		return nil
	}
	return &ChatContent{Parts: contentParts}
}

var mimeTypeMap = map[string]string{
	"json_object": "application/json",
	"text":        "text/plain",
}
