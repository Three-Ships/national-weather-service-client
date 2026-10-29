package nws_test

import (
	"encoding/json"
	"fmt"

	nws "github.com/Three-Ships/national-weather-service-client"
)

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
