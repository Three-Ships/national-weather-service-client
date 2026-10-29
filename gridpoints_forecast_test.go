package nws_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	nws "github.com/Three-Ships/national-weather-service-client"
)

func TestForecastSourceFormats(t *testing.T) {
	for _, path := range []string{"/gridpoints/LWX/97,71/forecast", "/gridpoints/LWX/97,71/forecast/hourly"} {
		t.Run(path, func(t *testing.T) {
			c := fixtureClient(t, "forecast", path)
			got, _, err := c.Gridpoints.GetForecast(context.Background(), path, url.Values{"units": {"us"}})
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

func ExampleGridpointForecastPeriod() {
	// Display both NWS formats without converting their values or units.
	formatValue := func(raw json.RawMessage, scalarUnit string) string {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			panic(err)
		}
		switch value := value.(type) {
		case nil:
			return "missing"
		case float64:
			return fmt.Sprintf("%g %s", value, scalarUnit)
		case string:
			return value
		default:
			var measurement nws.QuantitativeValue
			if err := json.Unmarshal(raw, &measurement); err != nil {
				panic(err)
			}
			if measurement.Value == nil {
				return "missing"
			}
			return fmt.Sprintf("%g %s", *measurement.Value, measurement.UnitCode)
		}
	}

	periods := []nws.GridpointForecastPeriod{
		{
			Temperature:     json.RawMessage(`32`),
			TemperatureUnit: "F",
			WindSpeed:       json.RawMessage(`"5 to 10 mph"`),
		},
		{
			Temperature: json.RawMessage(`{"value":0,"unitCode":"wmoUnit:degC"}`),
			WindSpeed:   json.RawMessage(`{"value":null,"unitCode":"wmoUnit:km_h-1"}`),
		},
	}
	for _, period := range periods {
		fmt.Println(formatValue(period.Temperature, period.TemperatureUnit))
		fmt.Println(formatValue(period.WindSpeed, ""))
	}
	// Output:
	// 32 F
	// 5 to 10 mph
	// 0 wmoUnit:degC
	// missing
}
