package gemini

import (
	"encoding/json"
	"strings"
	"testing"

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
