package nws_test

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	nws "github.com/Three-Ships/national-weather-service-client"
)

func TestPointDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/points/38.8977,-77.0365" {
			t.Errorf("path = %q", r.URL.EscapedPath())
		}
		http.ServeFile(w, r, "testdata/points/point.json")
	}))
	t.Cleanup(server.Close)
	c, err := nws.NewClient("test-reader", nws.WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := c.Points.Get(context.Background(), 38.897699, -77.036501)
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid coordinate reached HTTP")
	}))
	t.Cleanup(server.Close)
	c, err := nws.NewClient("test-reader", nws.WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
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
			if _, _, err := c.Points.Get(context.Background(), tt.lat, tt.lon); err == nil {
				t.Fatal("expected coordinate error")
			}
		})
	}
}

func ExamplePointsService_Get() {
	// A local server keeps this example independent of live weather data.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=300")
		io.WriteString(w, `{"type":"Feature","properties":{"gridId":"LWX","timeZone":"America/New_York","forecastGridData":"https://api.weather.gov/gridpoints/LWX/97,71"}}`)
	}))
	defer server.Close()
	client, err := nws.NewClient("example-reader/1.0 (weather@example.com)", nws.WithBaseURL(server.URL))
	if err != nil {
		panic(err)
	}
	point, response, err := client.Points.Get(context.Background(), 38.8977, -77.0365)
	if err != nil {
		panic(err)
	}
	fmt.Println(response.Header.Get("Cache-Control"))
	fmt.Println(point.Properties.TimeZone)
	fmt.Println(point.Properties.ForecastGridData)
	// Output:
	// public, max-age=300
	// America/New_York
	// https://api.weather.gov/gridpoints/LWX/97,71
}
