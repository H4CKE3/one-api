package vertexaikey

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	relaymeta "github.com/songquanpeng/one-api/relay/meta"
	relaymodel "github.com/songquanpeng/one-api/relay/model"
)

func TestGetRequestURL(t *testing.T) {
	adaptor := &Adaptor{}
	meta := &relaymeta.Meta{
		BaseURL:         "https://aiplatform.googleapis.com",
		ActualModelName: "gemini-2.0-flash",
		APIKey:          "vertex-key",
	}

	url, err := adaptor.GetRequestURL(meta)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	expected := "https://aiplatform.googleapis.com/v1/publishers/google/models/gemini-2.0-flash:generateContent?key=vertex-key"
	if url != expected {
		t.Fatalf("unexpected request url: got %q want %q", url, expected)
	}
}

func TestGetStreamRequestURL(t *testing.T) {
	adaptor := &Adaptor{}
	meta := &relaymeta.Meta{
		BaseURL:         "https://aiplatform.googleapis.com",
		ActualModelName: "gemini-2.0-flash",
		APIKey:          "vertex-key",
		IsStream:        true,
	}

	url, err := adaptor.GetRequestURL(meta)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	expected := "https://aiplatform.googleapis.com/v1/publishers/google/models/gemini-2.0-flash:streamGenerateContent?alt=sse&key=vertex-key"
	if url != expected {
		t.Fatalf("unexpected stream request url: got %q want %q", url, expected)
	}
}

func TestConvertRequestProducesVertexJSONShape(t *testing.T) {
	adaptor := &Adaptor{}
	temperature := 0.2
	topP := 0.9
	presencePenalty := 0.4
	frequencyPenalty := 0.3
	stop := []string{"STOP"}
	request := &relaymodel.GeneralOpenAIRequest{
		Model:            "gemini-2.0-flash",
		MaxTokens:        256,
		Temperature:      &temperature,
		TopP:             &topP,
		TopK:             40,
		N:                2,
		PresencePenalty:  &presencePenalty,
		FrequencyPenalty: &frequencyPenalty,
		Stop:             stop,
		ResponseFormat: &relaymodel.ResponseFormat{
			Type: "json_object",
		},
		Messages: []relaymodel.Message{
			{Role: "system", Content: "Answer tersely"},
			{Role: "user", Content: "Say hello"},
		},
	}

	converted, err := adaptor.ConvertRequest(nil, 0, request)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload, err := json.Marshal(converted)
	if err != nil {
		t.Fatalf("failed to marshal converted request: %v", err)
	}

	jsonBody := string(payload)
	for _, want := range []string{
		"\"systemInstruction\"",
		"\"safetySettings\"",
		"\"generationConfig\"",
		"\"responseMimeType\":\"application/json\"",
		"\"candidateCount\":2",
		"\"maxOutputTokens\":256",
		"\"stopSequences\":[\"STOP\"]",
	} {
		if !strings.Contains(jsonBody, want) {
			t.Fatalf("expected JSON body to contain %s, got %s", want, jsonBody)
		}
	}

	for _, unwanted := range []string{
		"\"system_instruction\"",
		"\"generation_config\"",
		"\"safety_settings\"",
		"\"function_declarations\"",
		"HARM_CATEGORY_CIVIC_INTEGRITY",
	} {
		if strings.Contains(jsonBody, unwanted) {
			t.Fatalf("expected JSON body not to contain %s, got %s", unwanted, jsonBody)
		}
	}
}

func TestConvertRequestKeepsGoogleSearchToolNative(t *testing.T) {
	adaptor := &Adaptor{}
	request := &relaymodel.GeneralOpenAIRequest{
		Model: "gemini-2.0-flash",
		Tools: []relaymodel.Tool{
			{Type: "google_search"},
		},
		Messages: []relaymodel.Message{
			{Role: "user", Content: "Search recent news"},
		},
	}

	converted, err := adaptor.ConvertRequest(nil, 0, request)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	payload, err := json.Marshal(converted)
	if err != nil {
		t.Fatalf("failed to marshal converted request: %v", err)
	}

	jsonBody := string(payload)
	if !strings.Contains(jsonBody, `"googleSearch":{}`) {
		t.Fatalf("expected JSON body to contain native googleSearch tool, got %s", jsonBody)
	}
	if strings.Contains(jsonBody, "functionDeclarations") {
		t.Fatalf("expected JSON body not to contain functionDeclarations, got %s", jsonBody)
	}
}

// TestLocalOneAPIVertexGemini 向本机已启动的 one-api 发一条 chat/completions，用于联调 Vertex（API Key）渠道。
// 勿把令牌写进仓库：在 shell 里设置 ONE_API_TEST_KEY 后再运行，例如：
//
//	$env:ONE_API_TEST_KEY="sk-你的令牌"; go test ./relay/adaptor/vertexaikey/ -run TestLocalOneAPIVertexGemini -v
//
// 可选环境变量：ONE_API_TEST_BASE（默认 http://127.0.0.1:3000）、ONE_API_TEST_MODEL（默认 gemini-2.5-flash）。
func TestLocalOneAPIVertexGemini(t *testing.T) {
	key := strings.TrimSpace(os.Getenv("ONE_API_TEST_KEY"))
	if key == "" {
		t.Skip("set ONE_API_TEST_KEY to run local one-api integration test")
	}

	base := strings.TrimSpace(os.Getenv("ONE_API_TEST_BASE"))
	if base == "" {
		base = "http://127.0.0.1:3000"
	}
	base = strings.TrimRight(base, "/")

	model := strings.TrimSpace(os.Getenv("ONE_API_TEST_MODEL"))
	if model == "" {
		model = "gemini-2.5-flash"
	}

	body, err := json.Marshal(map[string]any{
		"model":       model,
		"max_tokens":  32,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "user", "content": "Reply with the single word: ok"},
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	t.Logf("\n————————————\n输入print\n————————————\n%s", string(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	t.Logf("\n————————————\n输出print\n————————————\n%s", string(responseBody))

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %s, body: %s", resp.Status, string(responseBody))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Choices) == 0 {
		t.Fatal("no choices in response")
	}
	if strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		t.Fatal("empty assistant content")
	}

	t.Logf("\n————————————\nassistant print\n————————————\n%s", strings.TrimSpace(out.Choices[0].Message.Content))
}
