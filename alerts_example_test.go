package nws_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	nws "github.com/Three-Ships/national-weather-service-client"
)

func ExampleAlertsService_List() {
	// A local server supplies two pages so the example does not contact NWS.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.RequestURI() {
		case "/alerts?area=DC&status=actual":
			io.WriteString(w, `{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"event":"Winter Weather Advisory"}}],"pagination":{"next":"/alerts?area=DC&cursor=next&status=actual"}}`)
		case "/alerts?area=DC&cursor=next&status=actual":
			io.WriteString(w, `{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"event":"Wind Advisory"}}]}`)
		default:
			http.Error(w, "unexpected request: "+r.URL.String(), http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client, err := nws.NewClient("example-reader/1.0 (weather@example.com)",
		nws.WithRequestInterval(0), nws.WithBaseURL(server.URL))
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, err := client.Alerts.List(ctx, url.Values{"area": {"DC"}, "status": {"actual"}})
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
		var next nws.AlertCollection
		if err := client.Get(ctx, page.Pagination.Next, nil, &next); err != nil {
			panic(err)
		}
		page = &next
	}
	// Output:
	// Winter Weather Advisory
	// Wind Advisory
}
