package ratio

import (
	"math"
	"testing"

	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/relay/channeltype"
)

func geminiQuotaCostUSD(t *testing.T, model string, channelType int, promptTokens int, completionTokens int, groupRatio float64) float64 {
	t.Helper()
	quota, ok := CalculateGeminiTextQuota(model, channelType, promptTokens, completionTokens, groupRatio)
	if !ok {
		t.Fatalf("expected Gemini model %s to use Gemini text pricing", model)
	}
	return float64(quota) / config.QuotaPerUnit
}

func assertNearlyEqual(t *testing.T, got float64, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1.0/config.QuotaPerUnit {
		t.Fatalf("cost mismatch, got %.8f, want %.8f", got, want)
	}
}

func TestGeminiDeveloperAPIPricingExamples(t *testing.T) {
	// Source: https://ai.google.dev/pricing
	// Gemini 2.0 Flash: input $0.10 / 1M tokens, output $0.40 / 1M tokens.
	assertNearlyEqual(
		t,
		geminiQuotaCostUSD(t, "gemini-2.0-flash", channeltype.Gemini, 10000, 2000, 1),
		10000.0/1000000.0*0.10+2000.0/1000000.0*0.40,
	)

	// Source: https://ai.google.dev/pricing
	// Gemini 2.5 Flash: input $0.30 / 1M tokens, output $2.50 / 1M tokens.
	assertNearlyEqual(
		t,
		geminiQuotaCostUSD(t, "gemini-2.5-flash", channeltype.Gemini, 12000, 3000, 1),
		12000.0/1000000.0*0.30+3000.0/1000000.0*2.50,
	)

	// Source: https://ai.google.dev/pricing
	// Gemini 3 Flash Preview: input $0.50 / 1M tokens, output $3.00 / 1M tokens.
	assertNearlyEqual(
		t,
		geminiQuotaCostUSD(t, "gemini-3-flash-preview", channeltype.Gemini, 695, 45, 1),
		695.0/1000000.0*0.50+45.0/1000000.0*3.00,
	)
}

func TestGeminiVertexAIPricingExamples(t *testing.T) {
	// Source: https://cloud.google.com/vertex-ai/generative-ai/pricing
	// Gemini 2.0 Flash: input $0.15 / 1M tokens, output $0.60 / 1M tokens.
	assertNearlyEqual(
		t,
		geminiQuotaCostUSD(t, "gemini-2.0-flash", channeltype.VertexAIKey, 10000, 2000, 1),
		10000.0/1000000.0*0.15+2000.0/1000000.0*0.60,
	)

	// Source: https://cloud.google.com/vertex-ai/generative-ai/pricing
	// Gemini 1.5 Flash: input $0.075 / 1M tokens, output $0.30 / 1M tokens.
	assertNearlyEqual(
		t,
		geminiQuotaCostUSD(t, "gemini-1.5-flash", channeltype.VertexAIKey, 20000, 5000, 1),
		20000.0/1000000.0*0.075+5000.0/1000000.0*0.30,
	)

	// Source: https://cloud.google.com/vertex-ai/generative-ai/pricing
	// Gemini 1.5 Flash-8B: input $0.0375 / 1M tokens, output $0.15 / 1M tokens.
	assertNearlyEqual(
		t,
		geminiQuotaCostUSD(t, "gemini-1.5-flash-8b", channeltype.VertexAIKey, 40000, 10000, 1),
		40000.0/1000000.0*0.0375+10000.0/1000000.0*0.15,
	)

	// Source: https://cloud.google.com/vertex-ai/generative-ai/pricing
	// Gemini 1.5 Pro <= 128k context: input $1.25 / 1M tokens, output $5.00 / 1M tokens.
	assertNearlyEqual(
		t,
		geminiQuotaCostUSD(t, "gemini-1.5-pro", channeltype.VertexAIKey, 8000, 1200, 1),
		8000.0/1000000.0*1.25+1200.0/1000000.0*5.00,
	)
}

func TestGeminiLongContextPricingExamples(t *testing.T) {
	// Source: https://cloud.google.com/vertex-ai/generative-ai/pricing
	// Gemini 1.5 Pro > 128k context: input $2.50 / 1M tokens, output $10.00 / 1M tokens.
	assertNearlyEqual(
		t,
		geminiQuotaCostUSD(t, "gemini-1.5-pro", channeltype.VertexAIKey, 130000, 4000, 1),
		130000.0/1000000.0*2.50+4000.0/1000000.0*10.00,
	)

	// Source: https://ai.google.dev/pricing
	// Gemini 2.5 Pro > 200k context: input $2.50 / 1M tokens, output $15.00 / 1M tokens.
	assertNearlyEqual(
		t,
		geminiQuotaCostUSD(t, "gemini-2.5-pro", channeltype.Gemini, 220000, 6000, 1),
		220000.0/1000000.0*2.50+6000.0/1000000.0*15.00,
	)
}

func TestGeminiGroupRatioPricingExample(t *testing.T) {
	// Group ratio is a platform multiplier applied after provider token cost.
	assertNearlyEqual(
		t,
		geminiQuotaCostUSD(t, "gemini-2.0-flash", channeltype.VertexAIKey, 10000, 2000, 1.2),
		(10000.0/1000000.0*0.15+2000.0/1000000.0*0.60)*1.2,
	)
}
