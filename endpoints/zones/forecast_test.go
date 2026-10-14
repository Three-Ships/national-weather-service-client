package zones_test

import (
	"net/http"
	"testing"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/endpoints/zones"
	"github.com/Three-Ships/national-weather-service-client/nwstest"
)

func TestZoneForecast(t *testing.T) {
	c := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.EscapedPath() != "/zones/forecast/id%2Fwith%3Fquery/forecast" {
			t.Errorf("escaped path=%s", r.URL.EscapedPath())
		}
		return nwstest.Response([]byte(`{"type":"Feature","geometry":null,"properties":{"zone":"https://api.weather.gov/zones/forecast/DCZ001","updated":"2026-01-01T12:00:00+00:00","periods":[{"number":1,"name":"Today","detailedForecast":"Sunny."}]}}`)), nil
	})}))
	got, err := zones.GetForecast(t.Context(), c, "forecast", "id/with?query")
	if err != nil || got == nil || got.Properties.Zone != "https://api.weather.gov/zones/forecast/DCZ001" ||
		got.Properties.Updated != "2026-01-01T12:00:00+00:00" || len(got.Properties.Periods) != 1 ||
		got.Properties.Periods[0].Number != 1 || got.Properties.Periods[0].DetailedForecast != "Sunny." {
		t.Fatalf("forecast=%+v error=%v", got, err)
	}
}
