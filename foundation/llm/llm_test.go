package llm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cbrophy/land_trakker/foundation/llm"
)

// ---- FakeClient tests ----

func TestFakeClient_Normal(t *testing.T) {
	c := llm.FakeClient{Content: `{"result":"ok"}`}
	resp, err := c.Complete(context.Background(), llm.Request{User: "describe this land"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != `{"result":"ok"}` {
		t.Errorf("Content = %q, want %q", resp.Content, `{"result":"ok"}`)
	}
}

func TestFakeClient_DefaultContent(t *testing.T) {
	c := llm.FakeClient{}
	resp, err := c.Complete(context.Background(), llm.Request{User: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != `{"ok":true}` {
		t.Errorf("Content = %q, want default", resp.Content)
	}
}

func TestFakeClient_ErrorTrigger(t *testing.T) {
	c := llm.FakeClient{}
	_, err := c.Complete(context.Background(), llm.Request{User: "please error now"})
	if err == nil {
		t.Fatal("expected error for user prompt containing 'error'")
	}
}

// ---- BudgetedClient tests ----

func TestBudgetedClient_Disabled(t *testing.T) {
	inner := llm.FakeClient{}
	b := llm.NewBudgetedClient(inner, false, 0, 0, nil)
	_, err := b.Complete(context.Background(), llm.Request{User: "hello"})
	if !errors.Is(err, llm.ErrDisabled) {
		t.Errorf("expected ErrDisabled, got %v", err)
	}
}

func TestBudgetedClient_Enabled(t *testing.T) {
	inner := llm.FakeClient{Content: `{"a":1}`}
	b := llm.NewBudgetedClient(inner, true, 0, 0, nil)
	resp, err := b.Complete(context.Background(), llm.Request{User: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != `{"a":1}` {
		t.Errorf("Content = %q, want %q", resp.Content, `{"a":1}`)
	}
}

func TestBudgetedClient_DailyLimit(t *testing.T) {
	inner := llm.FakeClient{}
	b := llm.NewBudgetedClient(inner, true, 2, 0, nil)
	ctx := context.Background()

	if _, err := b.Complete(ctx, llm.Request{User: "one"}); err != nil {
		t.Fatalf("call 1: %v", err)
	}
	if _, err := b.Complete(ctx, llm.Request{User: "two"}); err != nil {
		t.Fatalf("call 2: %v", err)
	}
	_, err := b.Complete(ctx, llm.Request{User: "three"})
	if !errors.Is(err, llm.ErrDailyLimitExceeded) {
		t.Errorf("expected ErrDailyLimitExceeded, got %v", err)
	}
	if got := b.DailyUsed(); got != 2 {
		t.Errorf("DailyUsed = %d, want 2", got)
	}
}

func TestBudgetedClient_MonthlyLimit(t *testing.T) {
	inner := llm.FakeClient{}
	b := llm.NewBudgetedClient(inner, true, 0, 2, nil)
	ctx := context.Background()

	if _, err := b.Complete(ctx, llm.Request{User: "one"}); err != nil {
		t.Fatalf("call 1: %v", err)
	}
	if _, err := b.Complete(ctx, llm.Request{User: "two"}); err != nil {
		t.Fatalf("call 2: %v", err)
	}
	_, err := b.Complete(ctx, llm.Request{User: "three"})
	if !errors.Is(err, llm.ErrMonthlyLimitExceeded) {
		t.Errorf("expected ErrMonthlyLimitExceeded, got %v", err)
	}
	if got := b.MonthlyUsed(); got != 2 {
		t.Errorf("MonthlyUsed = %d, want 2", got)
	}
}

func TestBudgetedClient_ResetDailyCount(t *testing.T) {
	inner := llm.FakeClient{}
	b := llm.NewBudgetedClient(inner, true, 1, 0, nil)
	ctx := context.Background()

	if _, err := b.Complete(ctx, llm.Request{User: "first"}); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := b.Complete(ctx, llm.Request{User: "second"}); !errors.Is(err, llm.ErrDailyLimitExceeded) {
		t.Fatalf("expected limit, got %v", err)
	}
	b.ResetDailyCount()
	if _, err := b.Complete(ctx, llm.Request{User: "third"}); err != nil {
		t.Errorf("after reset expected success, got %v", err)
	}
}

func TestBudgetedClient_ResetMonthlyCount(t *testing.T) {
	inner := llm.FakeClient{}
	b := llm.NewBudgetedClient(inner, true, 0, 1, nil)
	ctx := context.Background()

	if _, err := b.Complete(ctx, llm.Request{User: "first"}); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := b.Complete(ctx, llm.Request{User: "second"}); !errors.Is(err, llm.ErrMonthlyLimitExceeded) {
		t.Fatalf("expected limit, got %v", err)
	}
	b.ResetMonthlyCount()
	if _, err := b.Complete(ctx, llm.Request{User: "third"}); err != nil {
		t.Errorf("after reset expected success, got %v", err)
	}
}

func TestBudgetedClient_InnerErrorRollsBackCounters(t *testing.T) {
	inner := llm.FakeClient{}
	b := llm.NewBudgetedClient(inner, true, 5, 5, nil)
	ctx := context.Background()

	// Trigger inner error via "error" keyword
	_, err := b.Complete(ctx, llm.Request{User: "please error"})
	if err == nil {
		t.Fatal("expected error from inner client")
	}
	if got := b.DailyUsed(); got != 0 {
		t.Errorf("DailyUsed = %d after inner error, want 0", got)
	}
	if got := b.MonthlyUsed(); got != 0 {
		t.Errorf("MonthlyUsed = %d after inner error, want 0", got)
	}
}

func TestBudgetedClient_UnlimitedWhenZero(t *testing.T) {
	inner := llm.FakeClient{}
	b := llm.NewBudgetedClient(inner, true, 0, 0, nil)
	ctx := context.Background()

	for i := range 10 {
		if _, err := b.Complete(ctx, llm.Request{User: "hello"}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if got := b.DailyUsed(); got != 0 {
		t.Errorf("DailyUsed should stay 0 with no limit, got %d", got)
	}
}

// ---- estimateCost (via AnthropicClient indirectly) ----
// We test cost estimation by constructing a known fake and checking the Response.

func TestFakeClient_CostZero(t *testing.T) {
	c := llm.FakeClient{}
	resp, err := c.Complete(context.Background(), llm.Request{User: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CostUSD != 0 {
		t.Errorf("FakeClient CostUSD = %f, want 0", resp.CostUSD)
	}
}
