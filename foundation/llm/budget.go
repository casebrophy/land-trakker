package llm

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// BudgetedClient wraps a Client with an enabled flag, daily and monthly call
// caps, and per-call cost logging. Zero values for daily/monthly limits mean
// unlimited. Pass nil for log to use slog.Default().
type BudgetedClient struct {
	inner        Client
	enabled      bool
	dailyLimit   int32
	monthlyLimit int32
	dailyUsed    atomic.Int32
	monthlyUsed  atomic.Int32
	log          *slog.Logger
}

// NewBudgetedClient constructs a BudgetedClient.
// dailyLimit and monthlyLimit are maximum call counts; 0 means no cap.
func NewBudgetedClient(inner Client, enabled bool, dailyLimit, monthlyLimit int, log *slog.Logger) *BudgetedClient {
	if log == nil {
		log = slog.Default()
	}
	return &BudgetedClient{
		inner:        inner,
		enabled:      enabled,
		dailyLimit:   int32(dailyLimit),
		monthlyLimit: int32(monthlyLimit),
		log:          log,
	}
}

// ResetDailyCount resets the in-memory daily usage counter. Call at midnight.
func (b *BudgetedClient) ResetDailyCount() { b.dailyUsed.Store(0) }

// ResetMonthlyCount resets the in-memory monthly usage counter. Call at month boundary.
func (b *BudgetedClient) ResetMonthlyCount() { b.monthlyUsed.Store(0) }

// DailyUsed returns the number of successful calls made today.
func (b *BudgetedClient) DailyUsed() int { return int(b.dailyUsed.Load()) }

// MonthlyUsed returns the number of successful calls made this month.
func (b *BudgetedClient) MonthlyUsed() int { return int(b.monthlyUsed.Load()) }

// Complete implements Client. It enforces the enabled flag and call budgets,
// delegates to the inner client, and logs per-call token usage and cost.
func (b *BudgetedClient) Complete(ctx context.Context, req Request) (Response, error) {
	if !b.enabled {
		return Response{}, ErrDisabled
	}

	if b.dailyLimit > 0 {
		if b.dailyUsed.Add(1) > b.dailyLimit {
			b.dailyUsed.Add(-1)
			return Response{}, ErrDailyLimitExceeded
		}
	}

	if b.monthlyLimit > 0 {
		if b.monthlyUsed.Add(1) > b.monthlyLimit {
			b.monthlyUsed.Add(-1)
			if b.dailyLimit > 0 {
				b.dailyUsed.Add(-1)
			}
			return Response{}, ErrMonthlyLimitExceeded
		}
	}

	resp, err := b.inner.Complete(ctx, req)
	if err != nil {
		if b.dailyLimit > 0 {
			b.dailyUsed.Add(-1)
		}
		if b.monthlyLimit > 0 {
			b.monthlyUsed.Add(-1)
		}
		return Response{}, err
	}

	b.log.InfoContext(ctx, "llm call",
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.CostUSD,
		"daily_used", b.DailyUsed(),
		"monthly_used", b.MonthlyUsed(),
	)

	return resp, nil
}
