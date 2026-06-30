package gemini

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	relaymodel "github.com/songquanpeng/one-api/relay/model"
)

func TestConvertRequestKeepsGoogleSearchToolNative(t *testing.T) {
	request := relaymodel.GeneralOpenAIRequest{
		Model: "gemini-2.0-flash",
		Tools: []relaymodel.Tool{
			{Type: "google_search"},
		},
		Messages: []relaymodel.Message{
			{Role: "user", Content: "Search recent news"},
		},
	}

	converted := ConvertRequest(request)
	payload, err := json.Marshal(converted)
	if err != nil {
		t.Fatalf("failed to marshal converted request: %v", err)
	}

	jsonBody := string(payload)
	if !strings.Contains(jsonBody, `"google_search":{}`) {
		t.Fatalf("expected JSON body to contain native google_search tool, got %s", jsonBody)
	}
	if strings.Contains(jsonBody, "function_declarations") {
		t.Fatalf("expected JSON body not to contain function_declarations, got %s", jsonBody)
	}
}

func TestConvertRequestConvertsOpenAIToolCallRoundTripToGeminiRoles(t *testing.T) {
	request := relaymodel.GeneralOpenAIRequest{
		Model: "gemini-3-flash-preview",
		Messages: []relaymodel.Message{
			{Role: "system", Content: "Reply with the final answer only."},
			{Role: "user", Content: "What is 2+3? Use the tool."},
			{
				Role: "assistant",
				ToolCalls: []relaymodel.Tool{
					{
						Id:   "call_add",
						Type: "function",
						Function: relaymodel.Function{
							Name:      "add_numbers",
							Arguments: `{"a":2,"b":3}`,
						},
					},
				},
			},
			{Role: "tool", ToolCallId: "call_add", Content: "5"},
		},
		Tools: []relaymodel.Tool{
			{
				Type: "function",
				Function: relaymodel.Function{
					Name:        "add_numbers",
					Description: "Add two integers.",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"a": map[string]any{"type": "integer"},
							"b": map[string]any{"type": "integer"},
						},
						"required": []any{"a", "b"},
					},
				},
			},
		},
	}

	converted := ConvertRequest(request)
	payload, err := json.Marshal(converted)
	if err != nil {
		t.Fatalf("failed to marshal converted request: %v", err)
	}

	jsonBody := string(payload)
	if strings.Contains(jsonBody, `"role":"assistant"`) || strings.Contains(jsonBody, `"role":"tool"`) {
		t.Fatalf("expected Gemini payload to avoid assistant/tool roles, got %s", jsonBody)
	}
	if !strings.Contains(jsonBody, `"role":"model"`) {
		t.Fatalf("expected assistant tool call to become model content, got %s", jsonBody)
	}
	if !strings.Contains(jsonBody, `"functionCall":{"name":"add_numbers","args":{"a":2,"b":3}}`) {
		t.Fatalf("expected assistant tool call to become Gemini functionCall, got %s", jsonBody)
	}
	if !strings.Contains(jsonBody, `"role":"user"`) || !strings.Contains(jsonBody, `"functionResponse":{"name":"add_numbers","response":{"result":"5"}}`) {
		t.Fatalf("expected tool result to become Gemini user functionResponse, got %s", jsonBody)
	}
}

func TestHandlerKeepsUsageAndReturnsDiagnosticTextForEmptyGeminiContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"candidates": [{
				"content": {"parts": []},
				"finishReason": "MAX_TOKENS",
				"safetyRatings": [{"category": "HARM_CATEGORY_DANGEROUS_CONTENT", "probability": "NEGLIGIBLE"}]
			}],
			"usageMetadata": {
				"promptTokenCount": 2626,
				"candidatesTokenCount": 21,
				"thoughtsTokenCount": 487,
				"totalTokenCount": 3134
			}
		}`)),
	}

	err, usage, responseText := Handler(c, resp, 2626, "gemini-3-flash-preview")

	if err != nil {
		t.Fatalf("expected empty visible Gemini content with usage to remain billable, got error: %+v", err)
	}
	if usage == nil {
		t.Fatal("expected usage to be preserved")
	}
	if usage.PromptTokens != 2626 || usage.CompletionTokens != 508 || usage.TotalTokens != 3134 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
	if !strings.Contains(responseText, "Gemini returned no visible content") ||
		!strings.Contains(responseText, "finishReason=MAX_TOKENS") ||
		!strings.Contains(responseText, "completionTokens=508") {
		t.Fatalf("expected diagnostic response text, got %q", responseText)
	}
	if !strings.Contains(w.Body.String(), `"content":""`) {
		t.Fatalf("expected API response content to remain empty, got %s", w.Body.String())
	}
}

func TestHandlerReturnsAllGeminiTextPartsForSavedResponseText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"candidates": [{
				"content": {"parts": [{"text": "first"}, {"text": "second"}]},
				"finishReason": "STOP"
			}],
			"usageMetadata": {
				"promptTokenCount": 10,
				"candidatesTokenCount": 2,
				"totalTokenCount": 12
			}
		}`)),
	}

	err, usage, responseText := Handler(c, resp, 10, "gemini-3-flash-preview")

	if err != nil {
		t.Fatalf("expected handler success, got error: %+v", err)
	}
	if usage == nil || usage.TotalTokens != 12 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
	if responseText != "firstsecond" {
		t.Fatalf("expected all text parts in saved response text, got %q", responseText)
	}
}
