package nwstest_test

import (
	"context"
	"fmt"
	"net/http"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/endpoints/points"
	"github.com/Three-Ships/national-weather-service-client/nwstest"
)

func ExampleTransport() {
	client, err := nws.NewClient("test-reader/1.0 (test@example.com)",
		nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
			return nwstest.Response([]byte(`{"type":"Feature","properties":{"gridId":"LWX"}}`)), nil
		})}))
	if err != nil {
		panic(err)
	}
	point, err := points.Get(context.Background(), client, 38.8977, -77.0365)
	if err != nil {
		panic(err)
	}
	fmt.Println(point.Properties.GridID)
	// Output: LWX
}
