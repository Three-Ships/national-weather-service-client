// Private retry calculations need same-package tests. Tests using nwstest use
// package nws_test to avoid an import cycle through the helper package.
package nws

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name, value string
		want        time.Duration
	}{
		{"seconds", "120", 120 * time.Second},
		{"zero", "0", 0},
		{"whitespace", " 120 ", 120 * time.Second},
		{"negative", "-1", 0},
		{"date", now.Add(time.Minute).Format(http.TimeFormat), time.Minute},
		{"past", now.Add(-time.Minute).Format(http.TimeFormat), 0},
		{"invalid", "tomorrow", 0},
		{"empty", "", 0},
		{"overflowing duration", "9223372036854775807", time.Duration(1<<63 - 1)},
		{"overflowing integer", "999999999999999999999999", time.Duration(1<<63 - 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRetryAfter(tt.value, now); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestFullJitterBackoffBounds(t *testing.T) {
	policy := RetryPolicy{InitialDelay: time.Second, MaxDelay: 5 * time.Second}
	tests := []struct {
		name   string
		retry  int
		policy RetryPolicy
		bound  time.Duration
	}{
		{"initial delay", 1, policy, time.Second},
		{"second retry", 2, policy, 2 * time.Second},
		{"third retry", 3, policy, 4 * time.Second},
		{"delay cap", 4, policy, 5 * time.Second},
		{"high retry count", 100, policy, 5 * time.Second},
		{"overflow", 100, RetryPolicy{InitialDelay: time.Duration(1 << 62), MaxDelay: time.Duration(1<<63 - 1)}, time.Duration(1<<63 - 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{retry: tt.policy}
			var first time.Duration
			var varied bool
			for i := 0; i < 100; i++ {
				got := c.backoff(tt.retry)
				if got < 0 || got >= tt.bound {
					t.Fatalf("retry %d: delay %s outside [0,%s)", tt.retry, got, tt.bound)
				}
				if i == 0 {
					first = got
				} else if got != first {
					varied = true
				}
			}
			if !varied {
				t.Fatal("backoff has no jitter")
			}
		})
	}
}
