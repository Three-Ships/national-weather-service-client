package alerts_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/endpoints/alerts"
	"github.com/Three-Ships/national-weather-service-client/nwstest"
)

func ExampleList() {
	// The transport supplies two pages so the example does not contact NWS.
	client, err := nws.NewClient("example-reader/1.0 (weather@example.com)",
		nws.WithRequestInterval(0),
		nws.WithHTTPClient(&http.Client{Transport: nwstest.Transport(func(r *http.Request) (*http.Response, error) {
			switch r.URL.RequestURI() {
			case "/alerts?area=DC&status=actual":
				return nwstest.Response([]byte(`{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"event":"Winter Weather Advisory"}}],"pagination":{"next":"https://api.weather.gov/alerts?area=DC&cursor=next&status=actual"}}`)), nil
			case "/alerts?area=DC&cursor=next&status=actual":
				return nwstest.Response([]byte(`{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"event":"Wind Advisory"}}]}`)), nil
			default:
				return nil, fmt.Errorf("unexpected request: %s", r.URL)
			}
		})}))
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, err := alerts.List(ctx, client, url.Values{"area": {"DC"}, "status": {"actual"}})
	if err != nil {
		panic(err)
	}
	for {
		for _, feature := range page.Features {
			fmt.Println(feature.Properties.Event)
		}
		if page.Pagination == nil || page.Pagination.Next == "" {
			break
		}
		// Next contains the continuation query; do not reapply initial filters.
		// Decode into a fresh collection so omitted metadata cannot remain stale.
		var next alerts.Collection
		if err := client.Get(ctx, page.Pagination.Next, nil, &next); err != nil {
			panic(err)
		}
		page = &next
	}
	// Output:
	// Winter Weather Advisory
	// Wind Advisory
}
