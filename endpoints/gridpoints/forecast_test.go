package gridpoints_test

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/endpoints/gridpoints"
	"github.com/Three-Ships/national-weather-service-client/nwstest"
)

func TestForecastSourceFormats(t *testing.T) {
	for _, path := range []string{"/gridpoints/LWX/97,71/forecast", "/gridpoints/LWX/97,71/forecast/hourly"} {
		t.Run(path, func(t *testing.T) {
			c := nwstest.FixtureClient(t, "forecast", path)
			got, err := gridpoints.GetForecast(context.Background(), c, path, url.Values{"units": {"us"}})
			if err != nil {
				t.Fatal(err)
			}
			f := got.Properties
			if f.UpdateTime != "2026-01-01T12:00:00Z" || len(f.Periods) != 2 || string(f.Periods[0].Temperature) != "32" || string(f.Periods[0].WindSpeed) != `"5 to 10 mph"` {
				t.Fatalf("forecast = %+v", f)
			}
			var temp nws.QuantitativeValue
			if err := json.Unmarshal(f.Periods[1].Temperature, &temp); err != nil {
				t.Fatal(err)
			}
			if temp.Value == nil || *temp.Value != 0 || temp.UnitCode != "wmoUnit:degC" || f.Periods[0].ProbabilityOfPrecipitation.Value != nil || f.Periods[1].Dewpoint == nil {
				t.Fatalf("periods = %+v", f.Periods)
			}
		})
	}
}
