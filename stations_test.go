package nws_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	nws "github.com/Three-Ships/national-weather-service-client"
)

func newStationsTestClient(t *testing.T, handler http.HandlerFunc) *nws.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := nws.NewClient("test-reader", nws.WithBaseURL(server.URL), nws.WithRequestInterval(0))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestStationMetadata(t *testing.T) {
	c := newStationsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gridpoints/LWX/97,71/stations" {
			t.Errorf("path = %s", r.URL.Path)
		}
		io.WriteString(w, `{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"stationIdentifier":"KDCA","name":"Washington","timeZone":"America/New_York","elevation":{"value":3,"unitCode":"wmoUnit:m"}}}],"pagination":{"next":"https://api.weather.gov/stations?cursor=next"},"observationStations":["https://api.weather.gov/stations/KDCA"]}`)
	})
	got, err := c.Stations.List(context.Background(), "/gridpoints/LWX/97,71/stations", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Features) != 1 || got.Features[0].Properties.StationIdentifier != "KDCA" ||
		got.Pagination == nil || got.Pagination.Next != "https://api.weather.gov/stations?cursor=next" ||
		len(got.ObservationStations) != 1 || got.ObservationStations[0] != "https://api.weather.gov/stations/KDCA" {
		t.Fatalf("stations=%+v", got)
	}
}

func TestSingleStation(t *testing.T) {
	c := newStationsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/stations/id%2Fwith%3Fquery" {
			t.Errorf("escaped path=%s", r.URL.EscapedPath())
		}
		io.WriteString(w, `{"type":"Feature","properties":{"stationIdentifier":"KDCA","name":"Washington","timeZone":"America/New_York","elevation":{"value":3,"unitCode":"wmoUnit:m"}}}`)
	})
	got, err := c.Stations.Get(t.Context(), "id/with?query")
	if err != nil || got == nil || got.Properties.StationIdentifier != "KDCA" || got.Properties.Elevation.Value == nil || *got.Properties.Elevation.Value != 3 {
		t.Fatalf("station=%+v error=%v", got, err)
	}
}

func TestStationsInputValidation(t *testing.T) {
	invalidIDs := []string{"", ".", ".."}
	for _, operation := range []struct {
		name    string
		values  []string
		request func(context.Context, *nws.Client, string) (bool, error)
	}{
		{
			name: "station metadata", values: invalidIDs,
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := client.Stations.Get(ctx, value)
				return result == nil, err
			},
		},
		{
			name: "observation history", values: invalidIDs,
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := client.Stations.ListObservations(ctx, value, nil)
				return result == nil, err
			},
		},
		{
			name: "latest observation", values: invalidIDs,
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := client.Stations.GetLatestObservation(ctx, value, nil)
				return result == nil, err
			},
		},
		{
			name: "observation by time: station ID", values: invalidIDs,
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := client.Stations.GetObservation(ctx, value, "2026-01-01T00:00:00Z")
				return result == nil, err
			},
		},
		{
			name: "observation by time: timestamp", values: []string{"", ".", "..", "latest", "2026-01-01", "2026-02-30T00:00:00Z"},
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := client.Stations.GetObservation(ctx, "KDCA", value)
				return result == nil, err
			},
		},
	} {
		t.Run(operation.name, func(t *testing.T) {
			for _, value := range operation.values {
				t.Run("input="+value, func(t *testing.T) {
					client := newStationsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
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
