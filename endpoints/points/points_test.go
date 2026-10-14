package points_test

import (
	"context"
	"math"
	"net/http"
	"testing"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/endpoints/points"
	"github.com/Three-Ships/national-weather-service-client/nwstest"
)

func TestPointDiscovery(t *testing.T) {
	c := nwstest.FixtureClient(t, "point", "/points/38.8977,-77.0365")
	got, err := points.Get(context.Background(), c, 38.897699, -77.036501)
	if err != nil {
		t.Fatal(err)
	}
	p := got.Properties
	if got.Type != "Feature" || len(got.Geometry) == 0 || p.Forecast == "" || p.ForecastHourly == "" || p.ForecastGridData == "" || p.ForecastZone != "https://api.weather.gov/zones/forecast/DCZ001" || p.TimeZone != "America/New_York" {
		t.Fatalf("point = %+v", got)
	}
	if p.AstronomicalData == nil || p.AstronomicalData.Sunrise == nil || *p.AstronomicalData.Sunrise != "2026-01-01T07:27:00-05:00" || p.AstronomicalData.NauticalTwilightBegin != nil {
		t.Fatalf("astronomy = %+v", p.AstronomicalData)
	}
	if p.RelativeLocation == nil || p.RelativeLocation.Properties.City != "Washington" || p.NWR == nil || p.NWR.Transmitter != nil {
		t.Fatalf("metadata = %+v", p)
	}
}

func TestInvalidCoordinates(t *testing.T) {
	c := nwstest.NewClient(t, nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
		t.Error("invalid coordinate reached HTTP")
		return nwstest.Response([]byte("{}")), nil
	})}))
	tests := []struct {
		name     string
		lat, lon float64
	}{
		{"latitude high", 90.01, 0}, {"latitude low", -90.01, 0},
		{"longitude high", 0, 180.01}, {"longitude low", 0, -180.01},
		{"latitude NaN", math.NaN(), 0}, {"longitude NaN", 0, math.NaN()},
		{"latitude infinity", math.Inf(1), 0}, {"longitude infinity", 0, math.Inf(-1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := points.Get(context.Background(), c, tt.lat, tt.lon); err == nil {
				t.Fatal("expected coordinate error")
			}
		})
	}
}
