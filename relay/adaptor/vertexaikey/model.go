package vertexaikey

type ChatRequest struct {
	CachedContent     string               `json:"cachedContent,omitempty"`
	Contents          []ChatContent        `json:"contents"`
	SystemInstruction *ChatContent         `json:"systemInstruction,omitempty"`
	Tools             []ChatTools          `json:"tools,omitempty"`
	SafetySettings    []ChatSafetySettings `json:"safetySettings,omitempty"`
	GenerationConfig  ChatGenerationConfig `json:"generationConfig,omitempty"`
	Labels            map[string]string    `json:"labels,omitempty"`
}

type ChatContent struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Text            string         `json:"text,omitempty"`
	InlineData      *InlineData    `json:"inlineData,omitempty"`
	FileData        *FileData      `json:"fileData,omitempty"`
	FunctionCall    *FunctionCall  `json:"functionCall,omitempty"`
	VideoMetadata   *VideoMetadata `json:"videoMetadata,omitempty"`
	MediaResolution string         `json:"mediaResolution,omitempty"`
}

type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type FileData struct {
	MimeType string `json:"mimeType"`
	FileURI  string `json:"fileUri"`
}

type FunctionCall struct {
	Name string `json:"name"`
	Args any    `json:"args"`
}

type VideoMetadata struct {
	StartOffset *Duration `json:"startOffset,omitempty"`
	EndOffset   *Duration `json:"endOffset,omitempty"`
	FPS         float64   `json:"fps,omitempty"`
}

type Duration struct {
	Seconds int64 `json:"seconds,omitempty"`
	Nanos   int32 `json:"nanos,omitempty"`
}

type ChatSafetySettings struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
	Method    string `json:"method,omitempty"`
}

type ChatTools struct {
	FunctionDeclarations any `json:"functionDeclarations,omitempty"`
}

type ThinkingConfig struct {
	ThinkingBudget int    `json:"thinkingBudget,omitempty"`
	ThinkingLevel  string `json:"thinkingLevel,omitempty"`
}

type ChatGenerationConfig struct {
	Temperature      *float64        `json:"temperature,omitempty"`
	TopP             *float64        `json:"topP,omitempty"`
	TopK             int             `json:"topK,omitempty"`
	CandidateCount   int             `json:"candidateCount,omitempty"`
	MaxOutputTokens  int             `json:"maxOutputTokens,omitempty"`
	PresencePenalty  *float64        `json:"presencePenalty,omitempty"`
	FrequencyPenalty *float64        `json:"frequencyPenalty,omitempty"`
	StopSequences    []string        `json:"stopSequences,omitempty"`
	ResponseMimeType string          `json:"responseMimeType,omitempty"`
	ResponseSchema   any             `json:"responseSchema,omitempty"`
	Seed             int             `json:"seed,omitempty"`
	ResponseLogprobs bool            `json:"responseLogprobs,omitempty"`
	Logprobs         int             `json:"logprobs,omitempty"`
	AudioTimestamp   bool            `json:"audioTimestamp,omitempty"`
	ThinkingConfig   *ThinkingConfig `json:"thinkingConfig,omitempty"`
	MediaResolution  string          `json:"mediaResolution,omitempty"`
}
