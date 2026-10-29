package nws

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetReturnsFinalResponse(t *testing.T) {
	calls, finals := 0, 0
	const body = `{"value":42}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/start" {
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, "/final?units=us", http.StatusFound)
			return
		}
		finals++
		if finals == 1 {
			w.Header().Set("ETag", `"retry"`)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("ETag", `"final"`)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	client := newTestClient(t, WithBaseURL(server.URL), WithRequestInterval(0))
	var result struct{ Value int }
	response, err := client.Get(t.Context(), "/start", nil, &result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != 42 || calls != 4 || response == nil || response.URL != server.URL+"/final?units=us" ||
		response.StatusCode != http.StatusCreated || response.Header.Get("Cache-Control") != "public, max-age=300" ||
		response.Header.Get("ETag") != `"final"` || string(response.Body) != body {
		t.Fatalf("result=%+v response=%+v calls=%d", result, response, calls)
	}
}

func TestGetRetainsResponseOnDecodeError(t *testing.T) {
	for _, tt := range []struct {
		name, body  string
		destination func() any
	}{
		{"malformed JSON", "{", func() any { return new(any) }},
		{"type mismatch", `{"value":"not a number"}`, func() any { return new(struct{ Value int }) }},
		{"nil destination", `{}`, func() any { return nil }},
		{"non-pointer destination", `{}`, func() any { return struct{}{} }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls, closed := 0, 0
			client := newTestClient(t, WithRequestInterval(0), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusOK,
					Header: http.Header{"Etag": {`"source"`}},
					Body:   trackedBody{Reader: strings.NewReader(tt.body), closed: &closed}}, nil
			})}))
			response, err := client.Get(t.Context(), "/custom", nil, tt.destination())
			var failure *RequestError
			if !errors.As(err, &failure) || failure.Retryable || failure.Attempts != 1 || calls != 1 || closed != 1 {
				t.Fatalf("error=%+v calls=%d closed=%d", failure, calls, closed)
			}
			if response == nil || response.StatusCode != http.StatusOK || response.URL != "https://api.weather.gov/custom" ||
				response.Header.Get("ETag") != `"source"` || string(response.Body) != tt.body {
				t.Fatalf("response=%+v", response)
			}
		})
	}
}

func TestResourceResponseFailureContract(t *testing.T) {
	transportErr := errors.New("connection reset")
	readErr := errors.New("incomplete body")
	for _, tt := range []struct {
		name                  string
		latitude              float64
		cancel                bool
		status                int
		body                  string
		transportErr, readErr error
		wantResponse          bool
		wantCalls             int
	}{
		{name: "invalid input", latitude: 91},
		{name: "canceled", cancel: true},
		{name: "HTTP failure", status: http.StatusForbidden, body: `{"detail":"forbidden"}`, wantCalls: 1},
		{name: "transport failure", transportErr: transportErr, wantCalls: 3},
		{name: "body failure", status: http.StatusOK, readErr: readErr, wantCalls: 3},
		{name: "decode failure", status: http.StatusOK, body: `{"properties":{"gridX":"invalid"}}`, wantCalls: 1, wantResponse: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := newTestClient(t, WithRequestInterval(0), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if tt.transportErr != nil {
					return nil, tt.transportErr
				}
				var reader io.Reader = strings.NewReader(tt.body)
				if tt.readErr != nil {
					reader = failingReader{tt.readErr}
				}
				return &http.Response{StatusCode: tt.status, Header: http.Header{"Cache-Control": {"no-cache"}}, Body: io.NopCloser(reader)}, nil
			})}))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.cancel {
				cancel()
			}
			result, response, err := client.Points.Get(ctx, tt.latitude, 0)
			if err == nil || result != nil || (response != nil) != tt.wantResponse || calls != tt.wantCalls {
				t.Fatalf("result=%+v response=%+v error=%v calls=%d", result, response, err, calls)
			}
			if tt.wantResponse && (response.Header.Get("Cache-Control") != "no-cache" || string(response.Body) != tt.body) {
				t.Fatalf("decode error lost response: %+v", response)
			}
			for _, cause := range []error{tt.transportErr, tt.readErr} {
				if cause != nil && !errors.Is(err, cause) {
					t.Fatalf("error=%v, want wrapped %v", err, cause)
				}
			}
			if tt.cancel && !errors.Is(err, context.Canceled) {
				t.Fatalf("error=%v, want cancellation", err)
			}
		})
	}
}
