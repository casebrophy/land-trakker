package llm

import (
	"context"
	"errors"
	"strings"
)

// FakeClient is a deterministic Client for tests and build-track operation.
// A User prompt containing "error" triggers a simulated error to exercise
// error-handling paths in callers.
type FakeClient struct {
	// Content is the string returned for non-error calls. Defaults to `{"ok":true}`.
	Content string
}

var errFakeError = errors.New("fake llm: simulated error")

// Complete implements Client.
func (f FakeClient) Complete(_ context.Context, req Request) (Response, error) {
	if strings.Contains(strings.ToLower(req.User), "error") {
		return Response{}, errFakeError
	}
	content := f.Content
	if content == "" {
		content = `{"ok":true}`
	}
	return Response{
		Content:      content,
		InputTokens:  len(req.User) / 4,
		OutputTokens: len(content) / 4,
		CostUSD:      0,
	}, nil
}
