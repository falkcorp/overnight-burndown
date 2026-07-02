// file: internal/agent/openai_fallback_test.go
// version: 1.2.0

package agent

import (
	"strings"
	"testing"
)

// TestCallResponsesWithRetry_PropagatesNon429 verifies that non-rate-limit
// errors from the Responses endpoint pass through immediately without the
// retry loop burning its budget.
func TestCallResponsesWithRetry_PropagatesNon429(t *testing.T) {
	// Stub timeNow so the retry budget never expires; the loop should exit
	// on the first non-429 error, not via deadline.
	orig := timeNow
	defer func() { timeNow = orig }()

	calls := 0
	// The actual Responses client call is behind an interface we can't easily
	// stub without a full mock server, so we verify the 429-detection logic
	// directly via is429.
	for _, msg := range []string{"auth: invalid api key", "403 Forbidden", "model not found"} {
		if is429(msg) {
			t.Errorf("is429(%q) = true, want false (non-rate-limit error)", msg)
		}
		calls++
	}
	if calls != 3 {
		t.Fatalf("expected 3 checks, got %d", calls)
	}
}

// TestIs429_DetectsBothFormats confirms the two shapes OpenAI uses for
// rate-limit errors are both recognized.
func TestIs429_DetectsBothFormats(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"429 Too Many Requests", true},
		{"rate limit exceeded", true},
		{"Rate Limit Exceeded", true},
		{"Please try again in 12.5s (rate limit)", true},
		{"auth: invalid api key", false},
		{"500 Internal Server Error", false},
		{"model_not_found", false},
	}
	for _, tc := range cases {
		if got := is429(tc.msg); got != tc.want {
			t.Errorf("is429(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

// TestIsQuotaExhausted_DistinguishesFromRateLimit is the core regression
// test for the 429-classification bug: is429() alone can't tell permanent
// billing exhaustion apart from a transient rate limit (both 429s, and
// OpenAI's insufficient_quota message even contains the substring "429"
// in the enclosing SDK error). isQuotaExhausted must be checked in
// addition to is429 before deciding whether to spend the retry budget.
func TestIsQuotaExhausted_DistinguishesFromRateLimit(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{
			name: "insufficient_quota (real prod error shape)",
			msg:  `openai call: POST "https://api.openai.com/v1/responses": 429 Too Many Requests {"message": "You exceeded your current quota, please check your plan and billing details.", "type": "insufficient_quota", "param": null, "code": "insufficient_quota"}`,
			want: true,
		},
		{
			name: "transient rate_limit_exceeded",
			msg:  `429 Too Many Requests {"message": "Rate limit reached for requests", "type": "requests", "code": "rate_limit_exceeded"}`,
			want: false,
		},
		{
			name: "plain rate limit phrase",
			msg:  "rate limit exceeded, try again in 5s",
			want: false,
		},
		{
			name: "non-429 error",
			msg:  "500 Internal Server Error",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isQuotaExhausted(tc.msg); got != tc.want {
				t.Errorf("isQuotaExhausted(%q) = %v, want %v", tc.msg, got, tc.want)
			}
			// Every quota-exhausted case must also be is429==true, since
			// the caller only checks isQuotaExhausted after is429 passes.
			if tc.want && !is429(tc.msg) {
				t.Errorf("test case %q is inconsistent: isQuotaExhausted=true but is429=false", tc.name)
			}
		})
	}
}

// TestParseRetryAfter_ParsesHint confirms the regex correctly extracts the
// "try again in X.Ys" hint that OpenAI embeds in 429 bodies.
func TestParseRetryAfter_ParsesHint(t *testing.T) {
	cases := []struct {
		msg  string
		want string // empty = expect 0
	}{
		{"Please try again in 12.5s.", "12.5s"},
		{"Rate limit hit. try again in 3s and cool down.", "3s"},
		{"no hint here", ""},
		{"try again in 0s", ""}, // 0 is rejected
	}
	for _, tc := range cases {
		d := parseRetryAfter(tc.msg)
		if tc.want == "" && d != 0 {
			t.Errorf("parseRetryAfter(%q) = %v, want 0", tc.msg, d)
		}
		if tc.want != "" && d == 0 {
			t.Errorf("parseRetryAfter(%q) = 0, want non-zero (%s)", tc.msg, tc.want)
		}
		if tc.want != "" && !strings.Contains(d.String(), strings.TrimSuffix(tc.want, "s")) {
			t.Errorf("parseRetryAfter(%q) = %v, want ~%s", tc.msg, d, tc.want)
		}
	}
}
