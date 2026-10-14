package stations_test

import (
	"context"
	"net/http"
	"testing"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/endpoints/stations"
	"github.com/Three-Ships/national-weather-service-client/nwstest"
)

func TestStationMetadata(t *testing.T) {
	c := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/gridpoints/LWX/97,71/stations" {
			t.Errorf("path = %s", r.URL.Path)
		}
		return nwstest.Response([]byte(`{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"stationIdentifier":"KDCA","name":"Washington","timeZone":"America/New_York","elevation":{"value":3,"unitCode":"wmoUnit:m"}}}],"pagination":{"next":"https://api.weather.gov/stations?cursor=next"},"observationStations":["https://api.weather.gov/stations/KDCA"]}`)), nil
	})}))
	got, err := stations.List(context.Background(), c, "/gridpoints/LWX/97,71/stations", nil)
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
	c := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.EscapedPath() != "/stations/id%2Fwith%3Fquery" {
			t.Errorf("escaped path=%s", r.URL.EscapedPath())
		}
		return nwstest.Response([]byte(`{"type":"Feature","properties":{"stationIdentifier":"KDCA","name":"Washington","timeZone":"America/New_York","elevation":{"value":3,"unitCode":"wmoUnit:m"}}}`)), nil
	})}))
	got, err := stations.Get(t.Context(), c, "id/with?query")
	if err != nil || got == nil || got.Properties.StationIdentifier != "KDCA" || got.Properties.Elevation.Value == nil || *got.Properties.Elevation.Value != 3 {
		t.Fatalf("station=%+v error=%v", got, err)
	}
}

func TestInputValidation(t *testing.T) {
	invalidIDs := []string{"", ".", ".."}
	for _, operation := range []struct {
		name    string
		values  []string
		request func(context.Context, *nws.Client, string) (bool, error)
	}{
		{
			name: "station metadata", values: invalidIDs,
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := stations.Get(ctx, client, value)
				return result == nil, err
			},
		},
		{
			name: "observation history", values: invalidIDs,
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := stations.ListObservations(ctx, client, value, nil)
				return result == nil, err
			},
		},
		{
			name: "latest observation", values: invalidIDs,
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := stations.GetLatestObservation(ctx, client, value, nil)
				return result == nil, err
			},
		},
		{
			name: "observation by time: station ID", values: invalidIDs,
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := stations.GetObservation(ctx, client, value, "2026-01-01T00:00:00Z")
				return result == nil, err
			},
		},
		{
			name: "observation by time: timestamp", values: []string{"", ".", "..", "latest", "2026-01-01", "2026-02-30T00:00:00Z"},
			request: func(ctx context.Context, client *nws.Client, value string) (bool, error) {
				result, err := stations.GetObservation(ctx, client, "KDCA", value)
				return result == nil, err
			},
		},
	} {
		t.Run(operation.name, func(t *testing.T) {
			for _, value := range operation.values {
				t.Run("input="+value, func(t *testing.T) {
					client := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
						t.Error("invalid input reached HTTP")
						return nwstest.Response([]byte("{}")), nil
					})}))
					isNil, err := operation.request(t.Context(), client, value)
					if !isNil || err == nil {
						t.Fatalf("input=%q: nil result=%t error=%v; want nil result and validation error", value, isNil, err)
					}
				})
			}
		})
	}
}
