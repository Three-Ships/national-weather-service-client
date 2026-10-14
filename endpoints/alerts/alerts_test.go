package alerts_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/endpoints/alerts"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *nws.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := nws.NewClient("test-reader", nws.WithBaseURL(server.URL), nws.WithRequestInterval(0))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestAlertsKeepSourceFields(t *testing.T) {
	for _, active := range []bool{false, true} {
		path := "/alerts"
		if active {
			path = "/alerts/active"
		}
		t.Run(path, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.EscapedPath() != path {
					t.Errorf("path = %q, want %q", r.URL.EscapedPath(), path)
				}
				http.ServeFile(w, r, "testdata/alerts.json")
			})
			var got *alerts.Collection
			var err error
			if active {
				got, err = alerts.ListActive(context.Background(), c, url.Values{"status": {"actual"}})
			} else {
				got, err = alerts.List(context.Background(), c, nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Title != "Current watches, warnings, and advisories" || got.Updated != "2026-01-01T12:00:00Z" || len(got.Features) != 1 || got.Pagination == nil || got.Pagination.Next == "" {
				t.Fatalf("collection = %+v", got)
			}
			alert := got.Features[0].Properties
			if alert.Onset != nil || alert.Ends != nil || alert.Headline != nil || alert.Event != "Winter Weather Advisory" || alert.Geocode.UGC[0] != "DCZ001" || alert.References[0].Identifier != "urn:oid:prior" || len(alert.Parameters["mixedSourceValues"]) != 3 {
				t.Fatalf("alert = %+v", alert)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var decoded alerts.Collection
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Title != got.Title || decoded.Updated != got.Updated ||
				decoded.Pagination == nil || decoded.Pagination.Next != got.Pagination.Next ||
				len(decoded.Features) != 1 || decoded.Features[0].Properties.ID != alert.ID {
				t.Fatalf("collection metadata changed during JSON round trip: %+v", decoded)
			}
		})
	}
}

func TestGet(t *testing.T) {
	for _, tt := range []struct {
		name, id, wantPath string
		wantErr            bool
	}{
		{name: "empty ID", wantErr: true},
		{name: "dot ID", id: ".", wantErr: true},
		{name: "parent ID", id: "..", wantErr: true},
		{name: "escaped ID", id: "id/with?query", wantPath: "/alerts/id%2Fwith%3Fquery"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if tt.wantErr {
					t.Error("invalid identifier reached HTTP")
				}
				if got := r.URL.EscapedPath(); got != tt.wantPath {
					t.Errorf("escaped path = %s, want %s", got, tt.wantPath)
				}
				io.WriteString(w, `{"type":"Feature","properties":{"id":"id/with?query"}}`)
			})
			got, err := alerts.Get(t.Context(), client, tt.id)
			if (err != nil) != tt.wantErr || (got == nil) != tt.wantErr {
				t.Fatalf("result=%+v error=%v; want error=%t", got, err, tt.wantErr)
			}
			if tt.wantErr {
				if calls != 0 {
					t.Fatalf("invalid identifier made %d HTTP attempts", calls)
				}
			} else if calls != 1 || got.Properties.ID != tt.id {
				t.Fatalf("calls=%d result=%+v; want one request and source ID %q", calls, got, tt.id)
			}
		})
	}
}

func TestActiveCounts(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/alerts/active/count" || !strings.Contains(r.Header.Get("Accept"), "application/ld+json") {
			t.Errorf("resource=%s Accept=%s", r.URL, r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/ld+json")
		io.WriteString(w, `{"total":3,"land":2,"marine":1,"regions":{"AL":1},"areas":{"DC":2},"zones":{"DCZ001":2}}`)
	})
	got, err := alerts.CountActive(t.Context(), c)
	if err != nil || got == nil || got.Total != 3 || got.Land != 2 || got.Marine != 1 ||
		got.Regions["AL"] != 1 || got.Areas["DC"] != 2 || got.Zones["DCZ001"] != 2 {
		t.Fatalf("counts=%+v error=%v", got, err)
	}
}

func TestRecognizedEventTypes(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/alerts/types" || !strings.Contains(r.Header.Get("Accept"), "application/ld+json") {
			t.Errorf("resource=%s Accept=%s", r.URL, r.Header.Get("Accept"))
		}
		io.WriteString(w, `{"eventTypes":["Winter Weather Advisory","Future Event"]}`)
	})
	got, err := alerts.ListTypes(t.Context(), c)
	if err != nil || got == nil || len(got.Types) != 2 || got.Types[1] != "Future Event" {
		t.Fatalf("types=%+v error=%v", got, err)
	}
}
