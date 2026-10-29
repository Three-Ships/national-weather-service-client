package nws_test

import (
	"io"
	"net/http"
	"testing"
)

func TestZoneForecast(t *testing.T) {
	c := newZonesTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/zones/forecast/id%2Fwith%3Fquery/forecast" {
			t.Errorf("escaped path=%s", r.URL.EscapedPath())
		}
		io.WriteString(w, `{"type":"Feature","geometry":null,"properties":{"zone":"https://api.weather.gov/zones/forecast/DCZ001","updated":"2026-01-01T12:00:00+00:00","periods":[{"number":1,"name":"Today","detailedForecast":"Sunny."}]}}`)
	})
	got, err := c.Zones.GetForecast(t.Context(), "forecast", "id/with?query")
	if err != nil || got == nil || got.Properties.Zone != "https://api.weather.gov/zones/forecast/DCZ001" ||
		got.Properties.Updated != "2026-01-01T12:00:00+00:00" || len(got.Properties.Periods) != 1 ||
		got.Properties.Periods[0].Number != 1 || got.Properties.Periods[0].DetailedForecast != "Sunny." {
		t.Fatalf("forecast=%+v error=%v", got, err)
	}
}
