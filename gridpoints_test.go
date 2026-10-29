package nws_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	nws "github.com/Three-Ships/national-weather-service-client"
)

func fixtureClient(t *testing.T, fixture, path string) *nws.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != path {
			t.Errorf("path = %q, want %q", r.URL.EscapedPath(), path)
		}
		http.ServeFile(w, r, "testdata/gridpoints/"+fixture+".json")
	}))
	t.Cleanup(server.Close)
	c, err := nws.NewClient("test-reader", nws.WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestGridSourceLayers(t *testing.T) {
	c := fixtureClient(t, "grid", "/gridpoints/LWX/97,71")
	got, _, err := c.Gridpoints.Get(context.Background(), "/gridpoints/LWX/97,71")
	if err != nil {
		t.Fatal(err)
	}
	g := got.Properties
	if g.UpdateTime != "2026-01-01T12:00:00+00:00" || g.GridID != "LWX" || len(g.Layers) != 3 {
		t.Fatalf("grid = %+v", g)
	}
	temp := g.Layers["temperature"]
	if temp.UOM != "wmoUnit:degC" || len(temp.Values) != 2 || temp.Values[0].Value == nil || *temp.Values[0].Value != 0 || temp.Values[1].Value != nil || temp.Values[0].ValidTime != "2026-01-01T12:00:00+00:00/PT2H" {
		t.Fatalf("temperature = %+v", temp)
	}
	if _, ok := g.Layers["windSpeed"]; ok {
		t.Fatal("absent layer fabricated")
	}
	if _, ok := g.Layers["futureNumericLayer"]; !ok {
		t.Fatal("new source layer lost")
	}
	if g.Weather == nil || g.Weather.Values[0].Value[0].Weather == nil || *g.Weather.Values[0].Value[0].Weather != "snow" || g.Weather.Values[0].Value[0].Intensity != nil {
		t.Fatalf("weather = %+v", g.Weather)
	}
	if g.Hazards == nil || g.Hazards.Values[0].Value[0].EventNumber != nil || g.Hazards.Values[0].Value[0].Phenomenon != "WW" {
		t.Fatalf("hazards = %+v", g.Hazards)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var decoded nws.Feature[nws.Gridpoint]
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Properties, decoded.Properties) || got.Type != decoded.Type {
		t.Fatalf("grid round trip changed source values")
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	var props map[string]json.RawMessage
	if err := json.Unmarshal(wire["properties"], &props); err != nil {
		t.Fatal(err)
	}
	if props["temperature"] == nil || props["Layers"] != nil {
		t.Fatal("grid did not restore flat wire format")
	}
	g.Layers["updateTime"] = nws.GridpointLayer[*float64]{}
	if _, err := json.Marshal(g); err == nil {
		t.Fatal("layer may not overwrite metadata")
	}
}

func TestGridRejectsWrongLayerShape(t *testing.T) {
	var g nws.Gridpoint
	if err := json.Unmarshal([]byte(`{"temperature":{"values":[{"validTime":"start/PT1H","value":"hot"}]}}`), &g); err == nil {
		t.Fatal("expected numeric layer decode error")
	}
}
