package nws

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPacingConcurrentCallsRetriesAndRedirects(t *testing.T) {
	var mu sync.Mutex
	var starts []time.Time
	client := newTestClient(t, WithRequestInterval(20*time.Millisecond), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		status := http.StatusOK
		headers := make(http.Header)
		if r.URL.Path == "/retry" {
			status = http.StatusServiceUnavailable
		}
		if r.URL.Path == "/redirect" {
			status = http.StatusMovedPermanently
			headers.Set("Location", "/final")
		}
		return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader("{}")), Request: r}, nil
	})}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for _, path := range []string{"/one", "/two", "/retry", "/redirect"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.Fetch(ctx, path, nil, "")
		}()
	}
	wg.Wait()
	if len(starts) != 7 {
		t.Fatalf("attempt count = %d, want 7", len(starts))
	}
	for i := 1; i < len(starts); i++ {
		// Transport invocation can lag admission slightly under scheduler load.
		if gap := starts[i].Sub(starts[i-1]); gap < 10*time.Millisecond {
			t.Errorf("attempt gap = %s", gap)
		}
	}
}

func TestCancellationDuringRedirectPacing(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Location", "/final")
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)
	client := newTestClient(t, WithBaseURL(server.URL), WithRequestInterval(time.Hour))
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_, err := client.Fetch(ctx, "/start", nil, "")
	var failure *RequestError
	if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &failure) ||
		failure.Attempts != 1 || failure.Retryable || calls.Load() != 1 {
		t.Fatalf("calls=%d error=%v; want cancellation before redirect is sent", calls.Load(), err)
	}
}

func TestCancellationDuringPacing(t *testing.T) {
	for _, tt := range []struct {
		name    string
		waiters int
	}{
		{name: "active timer", waiters: 1},
		{name: "timer and queued callers", waiters: 10},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			client := newTestClient(t, WithRequestInterval(time.Hour),
				WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					calls.Add(1)
					return testResponse([]byte("{}")), nil
				})}))
			if _, err := client.Fetch(t.Context(), "/test", nil, ""); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()
			done := make(chan error, tt.waiters)
			for range tt.waiters {
				go func() { _, err := client.Fetch(ctx, "/test", nil, ""); done <- err }()
			}
			for range tt.waiters {
				select {
				case err := <-done:
					var failure *RequestError
					if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &failure) || failure.Attempts != 0 {
						t.Fatalf("error=%v; want cancellation before Do", err)
					}
				case <-time.After(time.Second):
					t.Fatal("pacer waiter failed to cancel")
				}
			}
			if calls.Load() != 1 {
				t.Fatalf("pacer admitted %d attempts", calls.Load())
			}
		})
	}
}
