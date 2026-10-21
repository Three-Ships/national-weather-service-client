package points_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/endpoints/points"
)

func ExampleGet() {
	// A local server keeps this example independent of live weather data.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"type":"Feature","properties":{"gridId":"LWX","timeZone":"America/New_York","forecastGridData":"https://api.weather.gov/gridpoints/LWX/97,71"}}`)
	}))
	defer server.Close()
	client, err := nws.NewClient("example-reader/1.0 (weather@example.com)", nws.WithBaseURL(server.URL))
	if err != nil {
		panic(err)
	}
	point, err := points.Get(context.Background(), client, 38.8977, -77.0365)
	if err != nil {
		panic(err)
	}
	fmt.Println(point.Properties.TimeZone)
	fmt.Println(point.Properties.ForecastGridData)
	// Output:
	// America/New_York
	// https://api.weather.gov/gridpoints/LWX/97,71
}
