package typesafe

import (
	"net/http"
	"testing"
	"time"

	"github.com/unimtx/typesafe-sdk-go/option"
)

func TestRetryDefaultsAndIsolation(t *testing.T) {
	a, b := option.DefaultRetryPolicy(), option.DefaultRetryPolicy()
	a.HTTPStatuses[0] = 999
	if b.HTTPStatuses[0] != 408 {
		t.Fatal("default policies share status storage")
	}
	if b.MaxRetries != 2 || b.BackoffInitial != 500*time.Millisecond || b.BackoffMax != 5*time.Second || b.BackoffJitter != .25 {
		t.Fatalf("defaults: %+v", b)
	}
	b.HTTPStatuses = nil
	if retryableStatus(503, b) {
		t.Fatal("empty status slice did not disable status retries")
	}
	b.HTTPStatuses = []int{409}
	if !retryableStatus(409, b) || retryableStatus(503, b) {
		t.Fatal("custom status classification mismatch")
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 21, 7, 28, 0, 0, time.UTC)
	cases := []struct {
		headers http.Header
		want    time.Duration
		ok      bool
	}{
		{http.Header{"Retry-After": {"1.5"}}, 1500 * time.Millisecond, true},
		{http.Header{"Retry-After-Ms": {"250"}, "Retry-After": {"3"}}, 250 * time.Millisecond, true},
		{http.Header{"Retry-After-Ms": {"bad"}, "Retry-After": {"3"}}, 3 * time.Second, true},
		{http.Header{"Retry-After": {"Wed, 21 Oct 2026 07:28:05 GMT"}}, 5 * time.Second, true},
		{http.Header{"Retry-After": {"Wed, 21 Oct 2026 07:27:00 GMT"}}, 0, true},
		{http.Header{"Retry-After": {"-1"}}, 0, false},
		{http.Header{"retry-after-ms": {"25"}}, 25 * time.Millisecond, true},
	}
	for _, tc := range cases {
		got, ok := parseRetryAfter(tc.headers, now)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("parse %v: got %s,%v want %s,%v", tc.headers, got, ok, tc.want, tc.ok)
		}
	}
}

func TestRedactionIsCaseInsensitive(t *testing.T) {
	headers := http.Header{"authorization": {"Bearer secret"}, "SET-cookie": {"session=secret"}, "X-Visible": {"ok"}}
	redacted := redactHeaders(headers)
	if headerValue(redacted, "Authorization") != "[REDACTED]" || headerValue(redacted, "Set-Cookie") != "[REDACTED]" || headerValue(redacted, "X-Visible") != "ok" {
		t.Fatalf("redacted headers: %v", redacted)
	}
}

func TestRetryDelay(t *testing.T) {
	p := option.DefaultRetryPolicy()
	for attempt, want := range []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second} {
		if got := retryDelay(attempt, nil, p, time.Time{}, 0); got != want {
			t.Fatalf("attempt %d: %s", attempt, got)
		}
	}
	if got := retryDelay(0, nil, p, time.Time{}, 1); got != 375*time.Millisecond {
		t.Fatalf("jitter: %s", got)
	}
	if got := retryDelay(0, http.Header{"Retry-After": {"60"}}, p, time.Time{}, 1); got != time.Minute {
		t.Fatalf("retry-after: %s", got)
	}
	if got := retryDelay(0, http.Header{"Retry-After": {"61"}}, p, time.Time{}, 0); got != 500*time.Millisecond {
		t.Fatalf("cap fallback: %s", got)
	}
	p.BackoffInitial = 0
	if got := retryDelay(1_000_000_000, nil, p, time.Time{}, 0); got != 0 {
		t.Fatalf("zero backoff at large attempt: %s", got)
	}
}
