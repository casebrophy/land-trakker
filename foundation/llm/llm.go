package llm

import (
	"context"
	"errors"
)

// Client sends prompts to an LLM and returns structured responses.
type Client interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

// Request is a single-turn prompt with an optional system message.
type Request struct {
	System    string
	User      string
	MaxTokens int // 0 uses the default (1024)
}

// Response holds the LLM output and per-call billing metrics.
type Response struct {
	Content      string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}

var (
	// ErrDisabled is returned when the LLM client is disabled via config.
	ErrDisabled = errors.New("llm: client is disabled")
	// ErrDailyLimitExceeded is returned when the configured daily call cap is reached.
	ErrDailyLimitExceeded = errors.New("llm: daily call limit exceeded")
	// ErrMonthlyLimitExceeded is returned when the configured monthly call cap is reached.
	ErrMonthlyLimitExceeded = errors.New("llm: monthly call limit exceeded")
)
