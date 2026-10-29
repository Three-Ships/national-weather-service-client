package nws_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	nws "github.com/Three-Ships/national-weather-service-client"
)

func TestResourceServicesUseClientPolicy(t *testing.T) {
	for _, tt := range []struct {
		name, path string
		request    func(context.Context, *nws.Client) error
	}{
		{"points", "/api/points/38.8977,-77.0365", func(ctx context.Context, c *nws.Client) error {
			_, err := c.Points.Get(ctx, 38.8977, -77.0365)
			return err
		}},
		{"gridpoints", "/api/gridpoints/LWX/97,71", func(ctx context.Context, c *nws.Client) error {
			_, err := c.Gridpoints.Get(ctx, "gridpoints/LWX/97,71")
			return err
		}},
		{"alerts", "/api/alerts", func(ctx context.Context, c *nws.Client) error {
			_, err := c.Alerts.List(ctx, nil)
			return err
		}},
		{"stations", "/api/stations/KDCA", func(ctx context.Context, c *nws.Client) error {
			_, err := c.Stations.Get(ctx, "KDCA")
			return err
		}},
		{"zones", "/api/zones/forecast/DCZ001", func(ctx context.Context, c *nws.Client) error {
			_, err := c.Zones.Get(ctx, "forecast", "DCZ001")
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != tt.path || r.Header.Get("User-Agent") != "service-test/1.0" {
					t.Errorf("path=%s user-agent=%q", r.URL.Path, r.Header.Get("User-Agent"))
				}
				if calls == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				io.WriteString(w, `{}`)
			}))
			t.Cleanup(server.Close)
			client, err := nws.NewClient("service-test/1.0", nws.WithBaseURL(server.URL+"/api"),
				nws.WithHTTPClient(server.Client()), nws.WithRequestInterval(0),
				nws.WithRetryPolicy(nws.RetryPolicy{MaxAttempts: 2, InitialDelay: time.Nanosecond, MaxDelay: time.Nanosecond}))
			if err != nil {
				t.Fatal(err)
			}
			if err := tt.request(t.Context(), client); err != nil {
				t.Fatal(err)
			}
			if calls != 2 {
				t.Fatalf("HTTP attempts=%d, want 2", calls)
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if err := tt.request(ctx, client); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled operation error=%v", err)
			}
			if calls != 2 {
				t.Fatalf("canceled operation reached HTTP: attempts=%d", calls)
			}
		})
	}
}

func TestPointForecastDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/points/38.8977,-77.0365":
			fmt.Fprintf(w, `{"type":"Feature","properties":{"forecast":"http://%s/gridpoints/LWX/98,72/forecast"}}`, r.Host)
		case "/gridpoints/LWX/98,72/forecast":
			io.WriteString(w, `{"type":"Feature","properties":{"periods":[{"name":"Today","shortForecast":"Sunny"}]}}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client, err := nws.NewClient("discovery-test", nws.WithBaseURL(server.URL), nws.WithRequestInterval(0))
	if err != nil {
		t.Fatal(err)
	}
	point, err := client.Points.Get(t.Context(), 38.8977, -77.0365)
	if err != nil {
		t.Fatal(err)
	}
	forecast, err := client.Gridpoints.GetForecast(t.Context(), point.Properties.Forecast, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(forecast.Properties.Periods) != 1 || forecast.Properties.Periods[0].ShortForecast != "Sunny" {
		t.Fatalf("forecast=%+v", forecast)
	}
}
