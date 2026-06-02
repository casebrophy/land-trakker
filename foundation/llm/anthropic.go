package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const defaultMaxTokens = 1024

// costPerMillionTokens maps model name prefixes to [inputCost, outputCost] in USD/MTok.
var costPerMillionTokens = []struct {
	prefix string
	input  float64
	output float64
}{
	{"claude-haiku-4", 0.80, 4.00},
	{"claude-sonnet-4", 3.00, 15.00},
	{"claude-opus-4", 15.00, 75.00},
	{"claude-3-haiku", 0.25, 1.25},
	{"claude-3-5-sonnet", 3.00, 15.00},
	{"claude-3-sonnet", 3.00, 15.00},
	{"claude-3-opus", 15.00, 75.00},
}

// estimateCost returns an approximate USD cost for a call to the named model.
// Returns 0 when the model is unrecognised.
func estimateCost(model string, inputTokens, outputTokens int) float64 {
	lower := strings.ToLower(model)
	for _, entry := range costPerMillionTokens {
		if strings.HasPrefix(lower, entry.prefix) {
			return float64(inputTokens)/1_000_000*entry.input +
				float64(outputTokens)/1_000_000*entry.output
		}
	}
	return 0
}

// AnthropicClient is a Client backed by the Anthropic Messages API.
type AnthropicClient struct {
	inner anthropic.Client
	model string
}

// NewAnthropicClient creates an AnthropicClient using the given API key and model.
// If model is empty it defaults to "claude-haiku-4-5".
func NewAnthropicClient(apiKey, model string) *AnthropicClient {
	if model == "" {
		model = anthropic.ModelClaudeHaiku4_5
	}
	return &AnthropicClient{
		inner: anthropic.NewClient(option.WithAPIKey(apiKey)),
		model: model,
	}
}

// Complete implements Client. It calls the Anthropic Messages API and returns
// the first text block from the response along with token usage and estimated cost.
func (a *AnthropicClient) Complete(ctx context.Context, req Request) (Response, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: int64(maxTokens),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.User)),
		},
	}
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}

	msg, err := a.inner.Messages.New(ctx, params)
	if err != nil {
		return Response{}, fmt.Errorf("anthropic complete: %w", err)
	}

	var content string
	for _, block := range msg.Content {
		if block.Type == "text" {
			content = block.Text
			break
		}
	}

	inputTokens := int(msg.Usage.InputTokens)
	outputTokens := int(msg.Usage.OutputTokens)

	return Response{
		Content:      content,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      estimateCost(a.model, inputTokens, outputTokens),
	}, nil
}
