package zones_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/endpoints/zones"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *nws.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := nws.NewClient("test-reader", nws.WithBaseURL(server.URL), nws.WithRequestInterval(0))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestZoneMetadata(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zones/forecast/DCZ001" {
			t.Errorf("path = %s", r.URL.Path)
		}
		io.WriteString(w, `{"type":"Feature","properties":{"id":"DCZ001","type":"forecast","name":"District of Columbia","state":"DC","timeZone":["America/New_York"],"radarStation":null}}`)
	})
	got, err := zones.Get(context.Background(), c, "forecast", "DCZ001")
	if err != nil || got.Properties.ID != "DCZ001" || got.Properties.TimeZone[0] != "America/New_York" {
		t.Fatalf("zone=%+v error=%v", got, err)
	}
}

func TestInputValidation(t *testing.T) {
	for _, operation := range []struct {
		name    string
		request func(context.Context, *nws.Client, string) (bool, error)
	}{
		{
			name: "metadata: zone ID",
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := zones.Get(ctx, client, "forecast", value)
				return result == nil, err
			},
		},
		{
			name: "metadata: zone type",
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := zones.Get(ctx, client, value, "DCZ001")
				return result == nil, err
			},
		},
		{
			name: "list by type",
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := zones.ListByType(ctx, client, value, nil)
				return result == nil, err
			},
		},
		{
			name: "forecast: zone type",
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := zones.GetForecast(ctx, client, value, "DCZ001")
				return result == nil, err
			},
		},
		{
			name: "forecast: zone ID",
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := zones.GetForecast(ctx, client, "forecast", value)
				return result == nil, err
			},
		},
	} {
		t.Run(operation.name, func(t *testing.T) {
			for _, value := range []string{"", ".", ".."} {
				t.Run("input="+value, func(t *testing.T) {
					client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
						t.Error("invalid input reached HTTP")
						io.WriteString(w, "{}")
					})
					isNil, err := operation.request(t.Context(), client, value)
					if !isNil || err == nil {
						t.Fatalf("input=%q: nil result=%t error=%v; want nil result and validation error", value, isNil, err)
					}
				})
			}
		})
	}
}

func TestZoneListings(t *testing.T) {
	for _, tt := range []struct {
		name     string
		resource string
		request  func(*nws.Client) (*nws.FeatureCollection[zones.Zone], error)
	}{
		{
			name: "all types", resource: "/zones?area=DC%2CMD&include_geometry=true",
			request: func(c *nws.Client) (*nws.FeatureCollection[zones.Zone], error) {
				return zones.List(t.Context(), c, url.Values{"area": {"DC,MD"}, "include_geometry": {"true"}})
			},
		},
		{
			name: "one type", resource: "/zones/forecast?point=38.8977%2C-77.0365",
			request: func(c *nws.Client) (*nws.FeatureCollection[zones.Zone], error) {
				return zones.ListByType(t.Context(), c, "forecast", url.Values{"point": {"38.8977,-77.0365"}})
			},
		},
		{
			name: "escaped type", resource: "/zones/type%2Fwith%3Fquery",
			request: func(c *nws.Client) (*nws.FeatureCollection[zones.Zone], error) {
				return zones.ListByType(t.Context(), c, "type/with?query", nil)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.RequestURI() != tt.resource {
					t.Errorf("resource=%s, want %s", r.URL.RequestURI(), tt.resource)
				}
				io.WriteString(w, `{"type":"FeatureCollection","features":[{"type":"Feature","geometry":null,"properties":{"id":"DCZ001","type":"forecast","state":null,"timeZone":["America/New_York"],"radarStation":null}}]}`)
			})
			got, err := tt.request(c)
			if err != nil || got == nil || len(got.Features) != 1 || got.Features[0].Properties.ID != "DCZ001" ||
				got.Features[0].Properties.State != nil || got.Features[0].Properties.RadarStation != nil {
				t.Fatalf("zones=%+v error=%v", got, err)
			}
		})
	}
}
