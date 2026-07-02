// file: internal/agent/openai_retry_test.go
// version: 1.1.0

package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"Please try again in 3.881s. Visit ...", 3881 * time.Millisecond},
		{"please try again in 12s.", 12 * time.Second},
		{"try again in 0.5s", 500 * time.Millisecond},
		{"no hint here", 0},
		{"try again in 0s", 0},
	}
	for _, c := range cases {
		got := parseRetryAfter(c.in)
		if got != c.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestCallOpenAIWithRetry_QuotaExhaustedFailsFast is an end-to-end
// regression test (real HTTP round trip via httptest, not just the
// isQuotaExhausted unit test) for the 429-classification bug: hitting
// OpenAI's insufficient_quota error must return after exactly one call,
// not burn the retry budget the way a transient rate limit would.
func TestCallOpenAIWithRetry_QuotaExhaustedFailsFast(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": {"message": "You exceeded your current quota, please check your plan and billing details.", "type": "insufficient_quota", "code": "insufficient_quota"}}`))
	}))
	defer srv.Close()

	// WithMaxRetries(0) disables the SDK's own built-in transport-level
	// retry so this test isolates callOpenAIWithRetry's retry logic —
	// without it, the SDK's default retries happen before our code ever
	// sees the error, making call counts non-deterministic here.
	client := openai.NewClient(option.WithAPIKey("test"), option.WithBaseURL(srv.URL), option.WithMaxRetries(0))

	_, err := callOpenAIWithRetry(context.Background(), client, openai.ChatCompletionNewParams{
		Model:    openai.ChatModel("gpt-4.1"),
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hi")},
	})
	if err == nil {
		t.Fatal("expected error from quota-exhausted response")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("expected exactly 1 HTTP call (fail fast, no retry), got %d", got)
	}
}

// TestCallOpenAIWithRetry_TransientRateLimitStillRetries confirms the fix
// didn't over-correct: a genuine rate_limit_exceeded 429 must still retry,
// not fail fast like quota exhaustion does.
func TestCallOpenAIWithRetry_TransientRateLimitStillRetries(t *testing.T) {
	origAfter := timeAfter
	timeAfter = func(time.Duration) <-chan time.Time {
		c := make(chan time.Time, 1)
		c <- time.Now()
		return c
	}
	defer func() { timeAfter = origAfter }()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": {"message": "Rate limit reached for requests", "type": "requests", "code": "rate_limit_exceeded"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":0,"model":"gpt-4.1","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	// WithMaxRetries(0) disables the SDK's own built-in transport-level
	// retry so this test isolates callOpenAIWithRetry's retry logic —
	// without it, the SDK's default retries happen before our code ever
	// sees the error, making call counts non-deterministic here.
	client := openai.NewClient(option.WithAPIKey("test"), option.WithBaseURL(srv.URL), option.WithMaxRetries(0))

	_, err := callOpenAIWithRetry(context.Background(), client, openai.ChatCompletionNewParams{
		Model:    openai.ChatModel("gpt-4.1"),
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hi")},
	})
	if err != nil {
		t.Fatalf("expected eventual success after retries, got: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("expected 3 calls (2 transient failures + 1 success), got %d", got)
	}
}

func TestBackoffFor(t *testing.T) {
	base, max := 2*time.Second, 30*time.Second
	want := []time.Duration{
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		max,
		max,
	}
	for i, w := range want {
		got := backoffFor(i+1, base, max)
		if got != w {
			t.Errorf("backoffFor(%d) = %v, want %v", i+1, got, w)
		}
	}
}
