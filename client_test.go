package nws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// roundTripFunc injects stream failures that an HTTP test server cannot produce.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newTestClient(t testing.TB, options ...Option) *Client {
	t.Helper()
	defaults := []Option{WithRetryPolicy(RetryPolicy{MaxAttempts: 3, InitialDelay: time.Nanosecond, MaxDelay: time.Nanosecond})}
	c, err := NewClient("test-reader/1.0 (test@example.com)", append(defaults, options...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func testResponse(body []byte) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}
}

func ExampleClient_Get() {
	client, err := NewClient("example-reader/1.0 (weather@example.com)",
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return testResponse([]byte(`{"glossary":[{"term":"Dew Point","definition":"A temperature."}]}`)), nil
		})}))
	if err != nil {
		panic(err)
	}
	var result struct {
		Glossary []struct{ Term, Definition string }
	}
	if _, err := client.Get(context.Background(), "/glossary", nil, &result); err != nil {
		panic(err)
	}
	fmt.Println(result.Glossary[0].Term)
	// Output: Dew Point
}

func TestNewClientValidation(t *testing.T) {
	tests := []struct {
		name    string
		agent   string
		options []Option
		wantErr bool
	}{
		{"valid defaults", "reader (reader@example.com)", nil, false},
		{"empty user agent", "", []Option{WithBaseURL("https://api.weather.gov")}, true},
		{"blank user agent", "  ", []Option{WithBaseURL("https://api.weather.gov")}, true},
		{"header injection", "reader\r\nX-Bad: yes", []Option{WithBaseURL("https://api.weather.gov")}, true},
		{"invalid control character", "reader\x00", []Option{WithBaseURL("https://api.weather.gov")}, true},
		{"nil option", "reader", []Option{nil}, true},
		{"nil HTTP client", "reader", []Option{WithHTTPClient(nil)}, true},
		{"relative base", "reader", []Option{WithBaseURL("/api")}, true},
		{"unsupported scheme", "reader", []Option{WithBaseURL("ftp://weather.gov")}, true},
		{"invalid URL", "reader", []Option{WithBaseURL("http://%")}, true},
		{"credentials", "reader", []Option{WithBaseURL("https://secret@weather.gov")}, true},
		{"base query", "reader", []Option{WithBaseURL("https://weather.gov?x=y")}, true},
		{"base fragment", "reader", []Option{WithBaseURL("https://weather.gov#fragment")}, true},
		{"zero attempts", "reader", []Option{WithRetryPolicy(RetryPolicy{})}, true},
		{"reversed delays", "reader", []Option{WithRetryPolicy(RetryPolicy{MaxAttempts: 1, InitialDelay: time.Second, MaxDelay: time.Millisecond})}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewClient(tt.agent, tt.options...); (err != nil) != tt.wantErr {
				t.Fatalf("NewClient() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestGetRequests(t *testing.T) {
	for _, tt := range []struct {
		name      string
		resource  string
		query     url.Values
		wantQuery url.Values
	}{
		{
			name:      "replace matching query and retain others",
			resource:  "/glossary?term=old&keep=yes",
			query:     url.Values{"term": {"rain & snow"}},
			wantQuery: url.Values{"term": {"rain & snow"}, "keep": {"yes"}},
		},
		{
			name:      "preserve unknown parameters and literal values",
			resource:  "/glossary?future_filter=unknown-value&literal=%25ZZ%3Bvalue&repeat=one&repeat=two",
			query:     url.Values{"from_options": {"%ZZ;value"}},
			wantQuery: url.Values{"future_filter": {"unknown-value"}, "literal": {"%ZZ;value"}, "repeat": {"one", "two"}, "from_options": {"%ZZ;value"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/api/glossary" {
					t.Errorf("request = %s %s", r.Method, r.URL)
				}
				if r.Header.Get("User-Agent") != "test-reader/1.0 (test@example.com)" {
					t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
				}
				if !strings.Contains(r.Header.Get("Accept"), "application/geo+json") {
					t.Errorf("Accept = %q", r.Header.Get("Accept"))
				}
				if got := r.URL.Query(); got.Encode() != tt.wantQuery.Encode() {
					t.Errorf("query = %v, want %v", got, tt.wantQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"glossary":[{"term":"rain"}]}`)
			}))
			t.Cleanup(server.Close)
			client := newTestClient(t, WithBaseURL(server.URL+"/api"))
			before := tt.query.Encode()
			var got struct{ Glossary []struct{ Term string } }
			if _, err := client.Get(t.Context(), tt.resource, tt.query, &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Glossary) != 1 || got.Glossary[0].Term != "rain" {
				t.Fatalf("result = %+v", got)
			}
			if tt.query.Encode() != before {
				t.Fatal("caller query mutated")
			}
		})
	}
}

func TestFetchValidation(t *testing.T) {
	for _, tt := range []struct {
		name, resource string
		query          url.Values
		accept         string
	}{
		{name: "empty resource"},
		{name: "different scheme", resource: "http://api.weather.gov/api/points"},
		{name: "different origin", resource: "https://evil.example/api"},
		{name: "network-path reference", resource: "//evil.example/api"},
		{name: "outside prefix", resource: "https://api.weather.gov/outside"},
		{name: "parent navigation", resource: "../outside"},
		{name: "encoded parent navigation", resource: "https://api.weather.gov/api/%2e%2e/outside"},
		{name: "encoded separators and navigation", resource: "https://api.weather.gov/api/child%2F..%2F..%2Foutside"},
		{name: "credentials", resource: "https://user@api.weather.gov/api/points"},
		{name: "fragment", resource: "/points#fragment"},
		{name: "invalid path escape", resource: "%"},
		{name: "invalid query value escape", resource: "/alerts?area=%ZZ&status=actual"},
		{name: "invalid query key escape", resource: "/alerts?%ZZ=DC&status=actual"},
		{name: "incomplete query escape", resource: "/alerts?area=%2"},
		{name: "unescaped semicolon", resource: "/alerts?area=DC;MD&status=actual"},
		{name: "malformed absolute URL query", resource: "https://api.weather.gov/api/alerts?area=%ZZ"},
		{name: "override does not hide malformed input", resource: "/alerts?area=%ZZ", query: url.Values{"area": {"DC"}}},
		{name: "Accept header injection", resource: "/alerts", accept: "application/json\r\nX-Bad: yes"},
		{name: "Accept control character", resource: "/alerts", accept: "image/png\x00"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, WithBaseURL("https://api.weather.gov/api"),
				WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					t.Error("invalid input reached HTTP")
					return testResponse([]byte("{}")), nil
				})}))
			response, err := client.Fetch(t.Context(), tt.resource, tt.query, tt.accept)
			var failure *RequestError
			if response != nil || err == nil || errors.As(err, &failure) {
				t.Fatalf("response=%+v error=%v; want input validation error before HTTP", response, err)
			}
		})
	}
}

func TestHTTPRetryClassification(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		wantAttempts int
		retryable    bool
	}{
		{"bad request", 400, 1, false},
		{"forbidden", 403, 1, false},
		{"not found", 404, 1, false},
		{"request timeout is still permanent 4xx", 408, 1, false},
		{"rate limited", 429, 3, true},
		{"server error", 500, 3, true},
		{"bad gateway", 502, 3, true},
		{"unavailable", 503, 3, true},
		{"not modified", 304, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tt.status)
				io.WriteString(w, `{"type":"urn:noaa:nws:api:InvalidPoint","title":"Invalid point","status":404,"detail":"outside coverage","instance":"request:1","correlationId":"abc"}`)
			}))
			t.Cleanup(server.Close)
			c := newTestClient(t, WithBaseURL(server.URL))
			_, err := c.Fetch(context.Background(), "/points/1,2", nil, "")
			var requestErr *RequestError
			if !errors.As(err, &requestErr) {
				t.Fatalf("error type = %T: %v", err, err)
			}
			if calls != tt.wantAttempts || requestErr.Attempts != tt.wantAttempts || requestErr.StatusCode != tt.status || requestErr.Retryable != tt.retryable {
				t.Fatalf("calls = %d, error = %+v", calls, requestErr)
			}
			if tt.status != http.StatusNotModified && (requestErr.Problem.Type != "urn:noaa:nws:api:InvalidPoint" || requestErr.Problem.CorrelationID != "abc" || requestErr.Body == "") {
				t.Fatalf("problem lost: %+v", requestErr)
			}
		})
	}
}

type trackedBody struct {
	io.Reader
	closed *int
}

func (b trackedBody) Close() error { *b.closed++; return nil }

func TestTransportRetriesAndBodyOwnership(t *testing.T) {
	transportErr := errors.New("connection reset")
	calls, closed := 0, 0
	c := newTestClient(t, WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, transportErr
		}
		status := http.StatusServiceUnavailable
		body := "unavailable"
		if calls == 3 {
			status, body = http.StatusOK, `{"ok":true}`
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: trackedBody{Reader: strings.NewReader(body), closed: &closed}}, nil
	})}))
	var got struct{ OK bool }
	if _, err := c.Get(context.Background(), "/test", nil, &got); err != nil {
		t.Fatal(err)
	}
	if calls != 3 || closed != 2 || !got.OK {
		t.Fatalf("calls=%d closed=%d got=%+v", calls, closed, got)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestResponseFailures(t *testing.T) {
	readErr := errors.New("truncated stream")
	for _, tt := range []struct {
		name           string
		status         int
		body           string
		readErr        error
		wantAttempts   int
		wantBodyLength int
		retryable      bool
	}{
		{name: "incomplete success retries", status: 200, readErr: readErr, wantAttempts: 3, retryable: true},
		{name: "incomplete forbidden response is terminal", status: 403, readErr: readErr, wantAttempts: 1},
		{name: "malformed JSON", status: 200, body: "{", wantAttempts: 1},
		{name: "trailing JSON", status: 200, body: "{}{}", wantAttempts: 1},
		{name: "bounded HTML error", status: 403, body: strings.Repeat("x", 100000), wantAttempts: 1, wantBodyLength: 64 << 10},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls, closed := 0, 0
			client := newTestClient(t, WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				var reader io.Reader = strings.NewReader(tt.body)
				if tt.readErr != nil {
					reader = failingReader{tt.readErr}
				}
				return &http.Response{StatusCode: tt.status, Header: make(http.Header), Body: trackedBody{Reader: reader, closed: &closed}}, nil
			})}))
			var dst any
			_, err := client.Get(t.Context(), "/test", nil, &dst)
			var failure *RequestError
			if !errors.As(err, &failure) || failure.StatusCode != tt.status || failure.Attempts != tt.wantAttempts ||
				failure.Retryable != tt.retryable || calls != tt.wantAttempts || closed != tt.wantAttempts || len(failure.Body) != tt.wantBodyLength {
				t.Fatalf("calls=%d closed=%d error=%+v", calls, closed, failure)
			}
			if tt.readErr != nil && !errors.Is(err, tt.readErr) {
				t.Fatalf("error=%v, want wrapped %v", err, tt.readErr)
			}
		})
	}
}

func TestRawRepresentation(t *testing.T) {
	c := newTestClient(t, WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Accept") != "image/png" {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(strings.NewReader("PNG!"))}, nil
	})}))
	resp, err := c.Fetch(context.Background(), "/icons/land/day/skc", nil, "image/png")
	if err != nil || string(resp.Body) != "PNG!" || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("response=%+v error=%v", resp, err)
	}
}

func TestRedirects(t *testing.T) {
	tests := []struct {
		name, location string
		basePath       string
		status         int
		wantCalls      int
		wantErr        bool
	}{
		{"same origin", "/final", "", http.StatusMovedPermanently, 2, false},
		{"cross origin", "https://evil.example/final", "", http.StatusMovedPermanently, 1, true},
		{"malformed query", "/final?area=%ZZ&status=actual", "", http.StatusMovedPermanently, 1, true},
		{"redirect loop", "/start", "", http.StatusMovedPermanently, 4, true},
		{"missing location", "", "", http.StatusMovedPermanently, 1, true},
		{"credentials", "http://user@evil.example/final", "", http.StatusMovedPermanently, 1, true},
		{"outside base path", "/outside", "/api", http.StatusFound, 1, true},
		{"encoded parent navigation", "/api/%2e%2e/outside", "/api", http.StatusFound, 1, true},
		{"encoded separators and navigation", "/api/child%2F..%2F..%2Foutside", "/api", http.StatusFound, 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path == "/final" {
					io.WriteString(w, "{}")
					return
				}
				w.Header().Set("Location", tt.location)
				w.WriteHeader(tt.status)
			}))
			t.Cleanup(server.Close)
			// Redirect handling must leave the caller's HTTP client configuration intact.
			hc := server.Client()
			var callbackCalls int
			hc.CheckRedirect = func(r *http.Request, via []*http.Request) error {
				callbackCalls++
				return errors.New("caller's redirect policy")
			}
			c := newTestClient(t, WithBaseURL(server.URL+tt.basePath), WithHTTPClient(hc))
			response, err := c.Fetch(context.Background(), "/start", nil, "")
			if (err != nil) != tt.wantErr || (response == nil) != tt.wantErr || calls != tt.wantCalls || callbackCalls != 0 || hc.CheckRedirect == nil {
				t.Fatalf("calls=%d error=%v", calls, err)
			}
			if tt.wantErr {
				var failure *RequestError
				if !errors.As(err, &failure) || failure.StatusCode != tt.status || failure.Attempts != 1 || failure.Retryable {
					t.Fatalf("error=%v; want terminal redirect error after one Do call", err)
				}
			}
		})
	}
}

func TestRedirectsHaveAnIndependentLimit(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var calls, finalCalls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("User-Agent") == "" || r.Header.Get("Accept") != "application/json" {
					t.Errorf("request headers lost: %v", r.Header)
				}
				if r.URL.Path != "/api/final" {
					w.Header().Set("Location", "final")
					w.Header().Set("Retry-After", "3600")
					w.WriteHeader(status)
					return
				}
				finalCalls++
				if finalCalls == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
				}
				io.WriteString(w, "{}")
			}))
			t.Cleanup(server.Close)
			client := newTestClient(t, WithBaseURL(server.URL+"/api"),
				WithRetryPolicy(RetryPolicy{MaxAttempts: 2, InitialDelay: time.Nanosecond, MaxDelay: time.Nanosecond}))
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			response, err := client.Fetch(ctx, "/start", nil, "application/json")
			if err != nil || calls != 4 || response.URL != server.URL+"/api/final" {
				t.Fatalf("calls=%d response=%+v error=%v", calls, response, err)
			}
		})
	}
}

func TestTransportFailures(t *testing.T) {
	for _, tt := range []struct {
		name      string
		redirect  bool
		wantURL   string
		wantCalls int
	}{
		{"direct request", false, "https://api.weather.gov/start", 3},
		{"redirect preserves final URL", true, "https://api.weather.gov/final", 6},
	} {
		t.Run(tt.name, func(t *testing.T) {
			transportErr := errors.New("connection reset")
			calls := 0
			client := newTestClient(t, WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if tt.redirect && r.URL.Path == "/start" {
					response := testResponse(nil)
					response.StatusCode = http.StatusFound
					response.Header.Set("Location", "/final")
					return response, nil
				}
				return nil, transportErr
			})}))
			response, err := client.Fetch(t.Context(), "/start", nil, "")
			var failure *RequestError
			if response != nil || !errors.Is(err, transportErr) || !errors.As(err, &failure) ||
				failure.URL != tt.wantURL || failure.Attempts != 3 || !failure.Retryable || calls != tt.wantCalls {
				t.Fatalf("calls=%d response=%+v error=%v; want URL %s and three failed Do calls", calls, response, err, tt.wantURL)
			}
		})
	}
}

func TestThreeRedirectsInOneAttempt(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 3 {
			w.Header().Set("Location", "/hop/"+strconv.Itoa(calls))
			w.WriteHeader(http.StatusFound)
			return
		}
		io.WriteString(w, "{")
	}))
	t.Cleanup(server.Close)
	client := newTestClient(t, WithBaseURL(server.URL),
		WithRetryPolicy(RetryPolicy{MaxAttempts: 1, InitialDelay: time.Nanosecond, MaxDelay: time.Nanosecond}))
	var dst any
	_, err := client.Get(t.Context(), "/start", nil, &dst)
	var failure *RequestError
	if !errors.As(err, &failure) || failure.Attempts != 1 || failure.StatusCode != 200 ||
		failure.Retryable || failure.URL != server.URL+"/hop/3" || calls != 4 {
		t.Fatalf("calls=%d error=%v; want decode error at final URL after one Do", calls, err)
	}
}

func TestCancellation(t *testing.T) {
	t.Run("before request", func(t *testing.T) {
		var calls int
		c := newTestClient(t, WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { calls++; return nil, errors.New("unexpected request") })}))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := c.Fetch(ctx, "/test", nil, "")
		if !errors.Is(err, context.Canceled) || calls != 0 {
			t.Fatalf("calls=%d error=%v", calls, err)
		}
	})
	t.Run("during transport", func(t *testing.T) {
		entered := make(chan struct{})
		c := newTestClient(t, WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			close(entered)
			<-r.Context().Done()
			return nil, r.Context().Err()
		})}))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { _, err := c.Fetch(ctx, "/test", nil, ""); done <- err }()
		<-entered
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	})
}

func TestRawJSONPreservesUnknownFields(t *testing.T) {
	c := newTestClient(t, WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"properties":{"periods":[{"newField":42}]}}`))}, nil
	})}))
	var got struct {
		Properties struct{ Periods []json.RawMessage }
	}
	if _, err := c.Get(context.Background(), "/forecast", nil, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Properties.Periods) != 1 || !strings.Contains(string(got.Properties.Periods[0]), "newField") {
		t.Fatalf("result = %+v", got)
	}
}

func TestGetNegotiatesJSONFormats(t *testing.T) {
	for _, format := range []string{"application/geo+json", "application/json", "application/ld+json"} {
		t.Run(format, func(t *testing.T) {
			c := newTestClient(t, WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				quality := make(map[string]float64)
				for _, entry := range strings.Split(r.Header.Get("Accept"), ",") {
					mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(entry))
					if err != nil {
						t.Fatal(err)
					}
					q := 1.0
					if params["q"] != "" {
						q, err = strconv.ParseFloat(params["q"], 64)
						if err != nil {
							t.Fatal(err)
						}
					}
					quality[mediaType] = q
				}
				if quality["application/geo+json"] <= quality["application/ld+json"] ||
					quality["application/json"] <= quality["application/ld+json"] || quality[format] <= 0 {
					t.Errorf("Accept=%q; require all JSON formats and prefer feature envelopes to JSON-LD", r.Header.Get("Accept"))
				}
				response := testResponse([]byte(`{"value":42}`))
				response.Header.Set("Content-Type", format)
				if quality[format] <= 0 {
					response.StatusCode = http.StatusNotAcceptable
				}
				return response, nil
			})}))
			var result struct {
				Value int `json:"value"`
			}
			if _, err := c.Get(t.Context(), "/resource", nil, &result); err != nil || result.Value != 42 {
				t.Fatalf("result=%+v error=%v", result, err)
			}
		})
	}
}

func TestRetryAfterIsAMinimumWait(t *testing.T) {
	for _, value := range []string{"120", time.Now().Add(2 * time.Minute).UTC().Format(http.TimeFormat)} {
		t.Run(value, func(t *testing.T) {
			var calls atomic.Int32
			c := newTestClient(t, WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls.Add(1)
				return &http.Response{StatusCode: 503, Header: http.Header{"Retry-After": {value}}, Body: io.NopCloser(strings.NewReader("{}"))}, nil
			})}))
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			_, err := c.Fetch(ctx, "/test", nil, "")
			var terminal *RequestError
			if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &terminal) || terminal.Attempts != 1 || calls.Load() != 1 {
				t.Fatalf("Retry-After ignored: calls=%d error=%v", calls.Load(), err)
			}
		})
	}
}

// retryWaitContext observes cancellation-channel selection after the fake HTTP
// attempt returns. With no HTTP timeout or pacing delay, the next selection is
// the retry wait, after its timer has been created and its Err check has passed.
// This keeps cancellation tests out of the transport and pre-wait fast paths.
type retryWaitContext struct {
	context.Context
	afterAttempt atomic.Bool
	waiting      chan struct{}
}

func (c *retryWaitContext) Done() <-chan struct{} {
	if c.afterAttempt.Load() {
		select {
		case c.waiting <- struct{}{}:
		default:
		}
	}
	return c.Context.Done()
}

func TestCancellationDuringRetryWait(t *testing.T) {
	for _, tt := range []struct {
		name       string
		status     int
		retryAfter string
	}{
		{name: "exponential backoff"},
		{name: "Retry-After", status: http.StatusTooManyRequests, retryAfter: "3600"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			ctx := &retryWaitContext{Context: parent, waiting: make(chan struct{}, 1)}
			var calls atomic.Int32
			client := newTestClient(t,
				WithRetryPolicy(RetryPolicy{MaxAttempts: 3, InitialDelay: time.Hour, MaxDelay: time.Hour}),
				WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					calls.Add(1)
					defer ctx.afterAttempt.Store(true)
					if tt.status == 0 {
						return nil, errors.New("connection reset")
					}
					response := testResponse([]byte("{}"))
					response.StatusCode = tt.status
					response.Header.Set("Retry-After", tt.retryAfter)
					response.Header.Set("Location", "/final")
					response.Request = r
					return response, nil
				})}))
			done := make(chan error, 1)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				_, err := client.Fetch(ctx, "/test", nil, "")
				done <- err
			}()
			t.Cleanup(func() {
				cancel()
				select {
				case <-finished:
				case <-time.After(time.Second):
					t.Error("request did not stop after cancellation")
				}
			})
			select {
			case <-ctx.waiting:
			case err := <-done:
				t.Fatalf("request finished before entering retry wait: %v", err)
			case <-time.After(time.Second):
				t.Fatal("request did not enter retry wait")
			}
			cancel()
			select {
			case err := <-done:
				var failure *RequestError
				if !errors.Is(err, context.Canceled) || !errors.As(err, &failure) ||
					failure.Attempts != 1 || failure.Retryable || calls.Load() != 1 {
					t.Fatalf("calls=%d error=%v; want cancellation after one attempt", calls.Load(), err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation did not interrupt retry wait")
			}
		})
	}
}

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
