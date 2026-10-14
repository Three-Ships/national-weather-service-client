// Package nwstest supplies HTTP fakes and test clients for NWS resource operations.
// Use Transport with nws.WithHTTPClient to return scripted responses without
// contacting NWS, or FixtureClient to serve JSON from your testdata directory.
package nwstest

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	nws "github.com/Three-Ships/national-weather-service-client"
)

// Transport implements http.RoundTripper with a function so tests can replace
// HTTP requests without starting a server.
type Transport func(*http.Request) (*http.Response, error)

func (f Transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// NewClient disables pacing and uses three attempts with nanosecond backoff.
// Options override these defaults. Invalid configuration fails the test.
// It does not replace HTTP: supply nws.WithHTTPClient with a fake Transport or
// nws.WithBaseURL with a local test server to avoid contacting NWS.
func NewClient(t testing.TB, options ...nws.Option) *nws.Client {
	t.Helper()
	defaults := []nws.Option{
		nws.WithRequestInterval(0),
		nws.WithRetryPolicy(nws.RetryPolicy{MaxAttempts: 3, InitialDelay: time.Nanosecond, MaxDelay: time.Nanosecond}),
	}
	c, err := nws.NewClient("test-reader/1.0 (test@example.com)", append(defaults, options...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Response returns HTTP 200 with empty headers and a fresh body reader.
// Call it for each attempt; callers can set StatusCode and Header to script
// HTTP errors and retry hints. The client consuming the response closes its body.
func Response(body []byte) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}
}

// FixtureClient injects JSON responses and checks the escaped request path
// without contacting NWS. fixture is a basename under the calling package's
// testdata directory. Each request receives a fresh response body.
func FixtureClient(t testing.TB, fixture, path string) *nws.Client {
	t.Helper()
	body, err := os.ReadFile("testdata/" + fixture + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return NewClient(t, nws.WithHTTPClient(&http.Client{Transport: Transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.EscapedPath() != path {
			t.Errorf("path = %q, want %q", r.URL.EscapedPath(), path)
		}
		return Response(body), nil
	})}))
}
