package nws_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"testing"

	nws "github.com/Three-Ships/national-weather-service-client"
)

func TestObservationsKeepSourceFields(t *testing.T) {
	body, err := os.ReadFile("testdata/stations/observation.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name     string
		resource string
		request  func(*nws.Client) (*nws.Feature[nws.Observation], error)
	}{
		{
			name: "latest", resource: "/stations/KDCA/observations/latest?require_qc=true",
			request: func(c *nws.Client) (*nws.Feature[nws.Observation], error) {
				return c.Stations.GetLatestObservation(t.Context(), "KDCA", url.Values{"require_qc": {"true"}})
			},
		},
		{
			name: "timestamp", resource: "/stations/KDCA/observations/2026-01-01T07:00:00.123-05:00",
			request: func(c *nws.Client) (*nws.Feature[nws.Observation], error) {
				return c.Stations.GetObservation(t.Context(), "KDCA", "2026-01-01T07:00:00.123-05:00")
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := newStationsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.RequestURI() != tt.resource {
					t.Errorf("resource=%s, want %s", r.URL.RequestURI(), tt.resource)
				}
				w.Write(body)
			})
			got, err := tt.request(c)
			if err != nil {
				t.Fatal(err)
			}
			observation := got.Properties
			if observation.StationID != "KDCA" || observation.Timestamp != "2026-01-01T12:00:00+00:00" ||
				observation.Temperature.Value == nil || *observation.Temperature.Value != 0 ||
				observation.Temperature.UnitCode != "wmoUnit:degC" || observation.Temperature.QualityControl != "V" ||
				observation.WindGust.Value != nil || observation.Icon != nil || len(got.Geometry) == 0 {
				t.Fatalf("source fields changed: %+v", observation)
			}
			if len(observation.PresentWeather) != 1 || observation.PresentWeather[0].Intensity != nil ||
				observation.PresentWeather[0].Modifier != nil || observation.PresentWeather[0].RawString != "FG" ||
				len(observation.CloudLayers) != 1 || observation.CloudLayers[0].Amount != "BKN" ||
				observation.CloudLayers[0].Base.Value == nil || *observation.CloudLayers[0].Base.Value != 500 {
				t.Fatalf("METAR fields changed: %+v", observation)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var original, roundTrip any
			if err := json.Unmarshal(body, &original); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &roundTrip); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(original, roundTrip) {
				t.Fatalf("observation JSON round trip changed source fields:\n%s", encoded)
			}
		})
	}
}

func TestObservationHistory(t *testing.T) {
	body, err := os.ReadFile("testdata/stations/observation.json")
	if err != nil {
		t.Fatal(err)
	}
	collection := []byte(`{"type":"FeatureCollection","features":[` + string(body) + `],"pagination":{"next":"https://api.weather.gov/stations/KDCA/observations?cursor=next"}}`)
	c := newStationsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stations/KDCA/observations" || r.URL.Query().Get("start") != "2026-01-01T00:00:00Z" ||
			r.URL.Query().Get("end") != "2026-01-02T00:00:00Z" || r.URL.Query().Get("limit") != "10" || r.URL.Query().Get("cursor") != "next" {
			t.Errorf("history resource=%s", r.URL)
		}
		w.Write(collection)
	})
	got, err := c.Stations.ListObservations(t.Context(), "KDCA", url.Values{
		"start": {"2026-01-01T00:00:00Z"}, "end": {"2026-01-02T00:00:00Z"}, "limit": {"10"}, "cursor": {"next"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Features) != 1 || got.Features[0].Properties.StationID != "KDCA" || got.Pagination == nil || got.Pagination.Next == "" {
		t.Fatalf("history=%+v", got)
	}
}

func TestNullCloudLayers(t *testing.T) {
	c := newStationsTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"type":"Feature","properties":{"cloudLayers":null}}`)
	})
	got, err := c.Stations.GetLatestObservation(t.Context(), "KDCA", nil)
	if err != nil || got == nil || got.Properties.CloudLayers != nil {
		t.Fatalf("result=%+v error=%v", got, err)
	}
}
