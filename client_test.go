package nws_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/nwstest"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
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
		{"empty user agent", "", nws.WithMaxResponseBytes(32 << 20)},
		{"blank user agent", "  ", nws.WithMaxResponseBytes(32 << 20)},
		{"header injection", "reader\r\nX-Bad: yes", nws.WithMaxResponseBytes(32 << 20)},
		{"invalid control character", "reader\x00", nws.WithMaxResponseBytes(32 << 20)},
		{"nil option", "reader", nil},
		{"nil HTTP client", "reader", nws.WithHTTPClient(nil)},
		{"relative base", "reader", nws.WithBaseURL("/api")},
		{"unsupported scheme", "reader", nws.WithBaseURL("ftp://weather.gov")},
		{"invalid URL", "reader", nws.WithBaseURL("http://%")},
		{"credentials", "reader", nws.WithBaseURL("https://secret@weather.gov")},
		{"base query", "reader", nws.WithBaseURL("https://weather.gov?x=y")},
		{"base fragment", "reader", nws.WithBaseURL("https://weather.gov#fragment")},
		{"zero body limit", "reader", nws.WithMaxResponseBytes(0)},
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

type trackedBody struct {
	io.Reader
	closed *int
}

func (b trackedBody) Close() error { *b.closed++; return nil }

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
		{name: "incomplete success", status: 200, readErr: readErr, limit: 32, wantAttempts: 1, retryable: true},
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
		{"rate limited", 429, 1, true},
		{"server error", 500, 1, true},
		{"bad gateway", 502, 1, true},
		{"unavailable", 503, 1, true},
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

func TestRedirects(t *testing.T) {
	tests := []struct {
		name, location string
		wantAttempts   int
		wantErr        bool
	}{
		{"same origin", "/final", 1, true},
		{"cross origin", "https://evil.example/final", 1, true},
		{"malformed query", "/final?area=%ZZ&status=actual", 1, true},
		{"redirect loop", "/start", 1, true},
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
