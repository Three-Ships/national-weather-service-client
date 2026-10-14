package nws_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/endpoints/gridpoints"
	"github.com/Three-Ships/national-weather-service-client/endpoints/points"
	"github.com/Three-Ships/national-weather-service-client/nwstest"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func ExampleClient_Get() {
	client, err := nws.NewClient("example-reader/1.0 (weather@example.com)",
		nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
			return nwstest.Response([]byte(`{"glossary":[{"term":"Dew Point","definition":"A temperature."}]}`)), nil
		})}))
	if err != nil {
		panic(err)
	}
	var result struct {
		Glossary []struct{ Term, Definition string }
	}
	if err := client.Get(context.Background(), "/glossary", nil, &result); err != nil {
		panic(err)
	}
	fmt.Println(result.Glossary[0].Term)
	// Output: Dew Point
}

func TestNewClientValidation(t *testing.T) {
	tests := []struct {
		name   string
		agent  string
		option nws.Option
	}{
		{"empty user agent", "", nws.WithRequestInterval(0)},
		{"blank user agent", "  ", nws.WithRequestInterval(0)},
		{"header injection", "reader\r\nX-Bad: yes", nws.WithRequestInterval(0)},
		{"invalid control character", "reader\x00", nws.WithRequestInterval(0)},
		{"nil option", "reader", nil},
		{"nil HTTP client", "reader", nws.WithHTTPClient(nil)},
		{"relative base", "reader", nws.WithBaseURL("/api")},
		{"unsupported scheme", "reader", nws.WithBaseURL("ftp://weather.gov")},
		{"invalid URL", "reader", nws.WithBaseURL("http://%")},
		{"credentials", "reader", nws.WithBaseURL("https://secret@weather.gov")},
		{"base query", "reader", nws.WithBaseURL("https://weather.gov?x=y")},
		{"base fragment", "reader", nws.WithBaseURL("https://weather.gov#fragment")},
		{"negative interval", "reader", nws.WithRequestInterval(-1)},
		{"zero body limit", "reader", nws.WithMaxResponseBytes(0)},
		{"zero attempts", "reader", nws.WithRetryPolicy(nws.RetryPolicy{})},
		{"reversed delays", "reader", nws.WithRetryPolicy(nws.RetryPolicy{MaxAttempts: 1, InitialDelay: time.Second, MaxDelay: time.Millisecond})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := nws.NewClient(tt.agent, tt.option); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
	if _, err := nws.NewClient("reader (reader@example.com)"); err != nil {
		t.Fatal(err)
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
			client := nwstest.NewClient(t, nws.WithBaseURL(server.URL+"/api"))
			before := tt.query.Encode()
			var got struct{ Glossary []struct{ Term string } }
			if err := client.Get(t.Context(), tt.resource, tt.query, &got); err != nil {
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
			client := nwstest.NewClient(t, nws.WithBaseURL("https://api.weather.gov/api"),
				nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
					t.Error("invalid input reached HTTP")
					return nwstest.Response([]byte("{}")), nil
				})}))
			response, err := client.Fetch(t.Context(), tt.resource, tt.query, tt.accept)
			var failure *nws.RequestError
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
			c := nwstest.NewClient(t, nws.WithBaseURL(server.URL))
			_, err := c.Fetch(context.Background(), "/points/1,2", nil, "")
			var requestErr *nws.RequestError
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
	c := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
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
	if err := c.Get(context.Background(), "/test", nil, &got); err != nil {
		t.Fatal(err)
	}
	if calls != 3 || closed != 2 || !got.OK {
		t.Fatalf("calls=%d closed=%d got=%+v", calls, closed, got)
	}

	c = nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) { return nil, transportErr })}))
	_, err := c.Fetch(context.Background(), "/test", nil, "")
	var terminal *nws.RequestError
	if !errors.Is(err, transportErr) || !errors.As(err, &terminal) || !terminal.Retryable || terminal.Attempts != 3 {
		t.Fatalf("error = %+v", err)
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
		limit          int64
		wantAttempts   int
		wantBodyLength int
		retryable      bool
	}{
		{name: "incomplete success retries", status: 200, readErr: readErr, limit: 32, wantAttempts: 3, retryable: true},
		{name: "incomplete forbidden response is terminal", status: 403, readErr: readErr, limit: 32, wantAttempts: 1},
		{name: "oversize success", status: 200, body: "12345", limit: 4, wantAttempts: 1},
		{name: "malformed JSON", status: 200, body: "{", limit: 32, wantAttempts: 1},
		{name: "trailing JSON", status: 200, body: "{}{}", limit: 32, wantAttempts: 1},
		{name: "bounded HTML error", status: 403, body: strings.Repeat("x", 100000), limit: 32, wantAttempts: 1, wantBodyLength: 64 << 10},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls, closed := 0, 0
			client := nwstest.NewClient(t, nws.WithMaxResponseBytes(tt.limit),
				nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
					calls++
					var reader io.Reader = strings.NewReader(tt.body)
					if tt.readErr != nil {
						reader = failingReader{tt.readErr}
					}
					return &http.Response{StatusCode: tt.status, Header: make(http.Header), Body: trackedBody{Reader: reader, closed: &closed}}, nil
				})}))
			var dst any
			err := client.Get(t.Context(), "/test", nil, &dst)
			var failure *nws.RequestError
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
	c := nwstest.NewClient(t, nws.WithMaxResponseBytes(4), nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
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
		wantAttempts   int
		wantErr        bool
	}{
		{"same origin", "/final", 2, false},
		{"cross origin", "https://evil.example/final", 1, true},
		{"malformed query", "/final?area=%ZZ&status=actual", 1, true},
		{"redirect loop", "/start", 3, true},
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
				w.WriteHeader(301)
			}))
			t.Cleanup(server.Close)
			// Redirect handling must leave the caller's HTTP client configuration intact.
			hc := server.Client()
			var callbackCalls int
			hc.CheckRedirect = func(r *http.Request, via []*http.Request) error {
				callbackCalls++
				return errors.New("caller's redirect policy")
			}
			c := nwstest.NewClient(t, nws.WithBaseURL(server.URL), nws.WithHTTPClient(hc))
			response, err := c.Fetch(context.Background(), "/start", nil, "")
			if (err != nil) != tt.wantErr || (response == nil) != tt.wantErr || calls != tt.wantAttempts || callbackCalls != 0 || hc.CheckRedirect == nil {
				t.Fatalf("calls=%d error=%v", calls, err)
			}
			if tt.wantErr {
				var failure *nws.RequestError
				if !errors.As(err, &failure) || failure.StatusCode != 301 || failure.Attempts != tt.wantAttempts || failure.Retryable {
					t.Fatalf("error=%v; want terminal redirect error after %d attempts", err, tt.wantAttempts)
				}
			}
		})
	}
}

func TestCancellation(t *testing.T) {
	t.Run("before request", func(t *testing.T) {
		var calls int
		c := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) { calls++; return nil, errors.New("unexpected request") })}))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := c.Fetch(ctx, "/test", nil, "")
		if !errors.Is(err, context.Canceled) || calls != 0 {
			t.Fatalf("calls=%d error=%v", calls, err)
		}
	})
	t.Run("during transport", func(t *testing.T) {
		entered := make(chan struct{})
		c := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
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

func TestPacingConcurrentCallsRetriesAndRedirects(t *testing.T) {
	var mu sync.Mutex
	var starts []time.Time
	c := nwstest.NewClient(t, nws.WithRequestInterval(20*time.Millisecond), nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		status := 200
		headers := make(http.Header)
		if r.URL.Path == "/retry" {
			status = 503
		}
		if r.URL.Path == "/redirect" {
			status = 301
			headers.Set("Location", "/final")
		}
		return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader("{}")), Request: r}, nil
	})}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for _, path := range []string{"/one", "/two", "/retry", "/redirect"} {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			_, err := c.Fetch(ctx, path, nil, "")
			if path != "/retry" && err != nil {
				t.Errorf("%s: %v", path, err)
			}
		}(path)
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

func TestIndependentClientPacers(t *testing.T) {
	hc := &http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})}
	first := nwstest.NewClient(t, nws.WithHTTPClient(hc), nws.WithRequestInterval(time.Hour))
	second := nwstest.NewClient(t, nws.WithHTTPClient(hc), nws.WithRequestInterval(time.Hour))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, c := range []*nws.Client{first, second} {
		if _, err := c.Fetch(ctx, "/test", nil, ""); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRawJSONPreservesUnknownFields(t *testing.T) {
	c := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"properties":{"periods":[{"newField":42}]}}`))}, nil
	})}))
	var got struct {
		Properties struct{ Periods []json.RawMessage }
	}
	if err := c.Get(context.Background(), "/forecast", nil, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Properties.Periods) != 1 || !strings.Contains(string(got.Properties.Periods[0]), "newField") {
		t.Fatalf("result = %+v", got)
	}
}

func TestGetNegotiatesJSONFormats(t *testing.T) {
	for _, format := range []string{"application/geo+json", "application/json", "application/ld+json"} {
		t.Run(format, func(t *testing.T) {
			c := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
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
				response := nwstest.Response([]byte(`{"value":42}`))
				response.Header.Set("Content-Type", format)
				if quality[format] <= 0 {
					response.StatusCode = http.StatusNotAcceptable
				}
				return response, nil
			})}))
			var result struct {
				Value int `json:"value"`
			}
			if err := c.Get(t.Context(), "/resource", nil, &result); err != nil || result.Value != 42 {
				t.Fatalf("result=%+v error=%v", result, err)
			}
		})
	}
}

func TestRedirectDrainIsBounded(t *testing.T) {
	for _, tt := range []struct {
		name          string
		contentLength int64
	}{
		{name: "known large body", contentLength: 8192},
		{name: "unknown length", contentLength: -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := &io.LimitedReader{R: strings.NewReader(strings.Repeat("x", 8192)), N: 8192}
			closed := 0
			client := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/start" {
					return &http.Response{
						StatusCode:    http.StatusMovedPermanently,
						Header:        http.Header{"Location": {"/final"}},
						Body:          trackedBody{Reader: body, closed: &closed},
						ContentLength: tt.contentLength,
						Request:       r,
					}, nil
				}
				return nwstest.Response([]byte("{}")), nil
			})}))
			if _, err := client.Fetch(t.Context(), "/start", nil, ""); err != nil {
				t.Fatal(err)
			}
			if read := 8192 - body.N; read > 2<<10 || closed != 1 {
				t.Fatalf("redirect body read=%d, closed=%d; want at most 2 KiB and closed once", read, closed)
			}
		})
	}
}

func TestRedirectReusesConnection(t *testing.T) {
	var connections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			w.Header().Set("Location", "/final")
			w.WriteHeader(http.StatusMovedPermanently)
			io.WriteString(w, "moved")
			return
		}
		io.WriteString(w, "{}")
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	t.Cleanup(server.Close)

	client := nwstest.NewClient(t, nws.WithBaseURL(server.URL), nws.WithHTTPClient(server.Client()))
	if _, err := client.Fetch(t.Context(), "/start", nil, ""); err != nil {
		t.Fatal(err)
	}
	if got := connections.Load(); got != 1 {
		t.Fatalf("redirect opened %d connections, want one reused connection", got)
	}
}

func TestRetryAfterIsAMinimumWait(t *testing.T) {
	for _, value := range []string{"120", time.Now().Add(2 * time.Minute).UTC().Format(http.TimeFormat)} {
		t.Run(value, func(t *testing.T) {
			var calls atomic.Int32
			c := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
				calls.Add(1)
				return &http.Response{StatusCode: 503, Header: http.Header{"Retry-After": {value}}, Body: io.NopCloser(strings.NewReader("{}"))}, nil
			})}))
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			_, err := c.Fetch(ctx, "/test", nil, "")
			var terminal *nws.RequestError
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
		{name: "redirect Retry-After", status: http.StatusMovedPermanently, retryAfter: "3600"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			ctx := &retryWaitContext{Context: parent, waiting: make(chan struct{}, 1)}
			var calls atomic.Int32
			client := nwstest.NewClient(t,
				nws.WithRetryPolicy(nws.RetryPolicy{MaxAttempts: 3, InitialDelay: time.Hour, MaxDelay: time.Hour}),
				nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
					calls.Add(1)
					defer ctx.afterAttempt.Store(true)
					if tt.status == 0 {
						return nil, errors.New("connection reset")
					}
					response := nwstest.Response([]byte("{}"))
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
				var failure *nws.RequestError
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
			client := nwstest.NewClient(t, nws.WithRequestInterval(time.Hour),
				nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
					calls.Add(1)
					return nwstest.Response([]byte("{}")), nil
				})}))
			if _, err := client.Fetch(t.Context(), "/test", nil, ""); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()
			done := make(chan error, tt.waiters)
			for i := 0; i < tt.waiters; i++ {
				go func() { _, err := client.Fetch(ctx, "/test", nil, ""); done <- err }()
			}
			for i := 0; i < tt.waiters; i++ {
				select {
				case err := <-done:
					if !errors.Is(err, context.DeadlineExceeded) {
						t.Fatal(err)
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

func TestResourceOperations(t *testing.T) {
	// Keep one successful-request client across operations to verify that the
	// resource packages share its pacer, not just its request configuration.
	var starts []time.Time
	var resources []string
	client := nwstest.NewClient(t, nws.WithRequestInterval(20*time.Millisecond),
		nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
			starts = append(starts, time.Now())
			resources = append(resources, r.URL.RequestURI())
			if r.Header.Get("User-Agent") != "test-reader/1.0 (test@example.com)" ||
				!strings.Contains(r.Header.Get("Accept"), "application/geo+json") {
				t.Errorf("resource package bypassed configured headers: %v", r.Header)
			}
			return nwstest.Response([]byte("{}")), nil
		})}))
	for _, operation := range []struct {
		name, resource string
		request        func(context.Context, *nws.Client) (bool, error)
	}{
		{
			name: "point discovery", resource: "/points/38.8977,-77.0365",
			request: func(ctx context.Context, client *nws.Client) (bool, error) {
				result, err := points.Get(ctx, client, 38.8977, -77.0365)
				return result == nil, err
			},
		},
		{
			name: "raw grid data", resource: "/gridpoints/LWX/97,71",
			request: func(ctx context.Context, client *nws.Client) (bool, error) {
				result, err := gridpoints.Get(ctx, client, "/gridpoints/LWX/97,71")
				return result == nil, err
			},
		},
	} {
		t.Run(operation.name, func(t *testing.T) {
			t.Run("request policy", func(t *testing.T) {
				before := len(starts)
				isNil, err := operation.request(t.Context(), client)
				if isNil || err != nil {
					t.Fatalf("nil result=%t error=%v; want successful result", isNil, err)
				}
				if len(starts) != before+1 || resources[len(resources)-1] != operation.resource {
					t.Fatalf("resource requests = %v, want one additional request to %s", resources, operation.resource)
				}
				if before > 0 {
					// Transport invocation can lag admission slightly under scheduler load.
					if gap := starts[before].Sub(starts[before-1]); gap < 10*time.Millisecond {
						t.Errorf("resource packages did not share the pacer: gap = %s", gap)
					}
				}
			})
			for _, response := range []struct {
				name   string
				status int
				body   string
			}{
				{name: "HTTP error", status: http.StatusNotFound, body: "{}"},
				{name: "invalid JSON", status: http.StatusOK, body: "{"},
			} {
				t.Run(response.name, func(t *testing.T) {
					attempts := 0
					client := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
						attempts++
						result := nwstest.Response([]byte(response.body))
						result.StatusCode = response.status
						return result, nil
					})}))
					isNil, err := operation.request(t.Context(), client)
					var failure *nws.RequestError
					if !isNil || !errors.As(err, &failure) || failure.StatusCode != response.status ||
						failure.Retryable || failure.Attempts != 1 || attempts != 1 {
						t.Fatalf("nil result=%t, attempts=%d, error=%v; want nil result and non-retryable HTTP %d RequestError", isNil, attempts, err, response.status)
					}
				})
			}
		})
	}
}
