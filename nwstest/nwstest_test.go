package nwstest_test

import (
	"errors"
	"github.com/Three-Ships/national-weather-service-client/nwstest"
	"io"
	"net/http"
	"testing"
)

func TestTransport(t *testing.T) {
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.weather.gov/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	response := nwstest.Response([]byte("body"))
	defer response.Body.Close()
	failure := errors.New("transport failed")
	for _, tt := range []struct {
		name     string
		response *http.Response
		err      error
	}{
		{name: "response", response: response},
		{name: "error", err: failure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			transport := nwstest.Transport(func(got *http.Request) (*http.Response, error) {
				if got != request {
					t.Error("transport replaced the request")
				}
				return tt.response, tt.err
			})
			got, err := transport.RoundTrip(request)
			if got != tt.response || !errors.Is(err, tt.err) {
				t.Fatalf("response=%v error=%v", got, err)
			}
		})
	}
}

func TestResponse(t *testing.T) {
	body := []byte(`{"ok":true}`)
	first := nwstest.Response(body)
	defer first.Body.Close()
	first.StatusCode = http.StatusServiceUnavailable
	first.Header.Set("Retry-After", "1")
	if _, err := io.ReadAll(first.Body); err != nil {
		t.Fatal(err)
	}

	second := nwstest.Response(body)
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK || len(second.Header) != 0 {
		t.Fatalf("response reused status or headers: %+v", second)
	}
	got, err := io.ReadAll(second.Body)
	if err != nil || string(got) != string(body) {
		t.Fatalf("body=%s error=%v", got, err)
	}
}

func TestFixtureClient(t *testing.T) {
	client := nwstest.FixtureClient(t, "response", "/escaped%2Fsegment")
	for range 2 {
		var result struct{ OK bool }
		if err := client.Get(t.Context(), "/escaped%2Fsegment", nil, &result); err != nil || !result.OK {
			t.Fatalf("result=%+v error=%v", result, err)
		}
	}
}
