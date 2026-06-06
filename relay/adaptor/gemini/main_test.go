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
