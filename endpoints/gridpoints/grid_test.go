package gridpoints_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/endpoints/gridpoints"
	"github.com/Three-Ships/national-weather-service-client/nwstest"
)

func TestGridSourceLayers(t *testing.T) {
	c := nwstest.FixtureClient(t, "grid", "/gridpoints/LWX/97,71")
	got, err := gridpoints.Get(context.Background(), c, "https://api.weather.gov/gridpoints/LWX/97,71")
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
	var decoded nws.Feature[gridpoints.Gridpoint]
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
	g.Layers["updateTime"] = gridpoints.Layer[*float64]{}
	if _, err := json.Marshal(g); err == nil {
		t.Fatal("layer may not overwrite metadata")
	}
}

func TestGridRejectsWrongLayerShape(t *testing.T) {
	var g gridpoints.Gridpoint
	if err := json.Unmarshal([]byte(`{"temperature":{"values":[{"validTime":"start/PT1H","value":"hot"}]}}`), &g); err == nil {
		t.Fatal("expected numeric layer decode error")
	}
}
