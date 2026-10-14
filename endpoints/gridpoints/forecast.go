package gridpoints

import (
	"context"
	"encoding/json"
	"net/url"

	nws "github.com/Three-Ships/national-weather-service-client"
)

// Forecast models the shared properties of the NWS OpenAPI Gridpoint12hForecast
// and GridpointHourlyForecast schemas. nws.Feature[Forecast] supplies the GeoJSON
// envelope for Gridpoint12hForecastGeoJson or GridpointHourlyForecastGeoJson.
//
// Schemas:
// https://api.weather.gov/openapi.json#/components/schemas/Gridpoint12hForecast
// https://api.weather.gov/openapi.json#/components/schemas/GridpointHourlyForecast
type Forecast struct {
	Units             string                `json:"units"`
	ForecastGenerator string                `json:"forecastGenerator"`
	GeneratedAt       string                `json:"generatedAt"`
	UpdateTime        string                `json:"updateTime"`
	ValidTimes        string                `json:"validTimes"`
	Elevation         nws.QuantitativeValue `json:"elevation"`
	Periods           []ForecastPeriod      `json:"periods"`
}

// ForecastPeriod combines the NWS OpenAPI Gridpoint12hForecastPeriod and
// GridpointHourlyForecastPeriod schemas. Dewpoint and RelativeHumidity are
// hourly-only fields. It uses raw JSON for fields whose format depends on NWS
// feature flags: Temperature is an integer or nws.QuantitativeValue; WindSpeed is
// a string or nws.QuantitativeValue; WindGust also permits null. Keeping these
// fields raw leaves callers free to choose a format without converting values
// or units.
//
// Schemas:
// https://api.weather.gov/openapi.json#/components/schemas/Gridpoint12hForecastPeriod
// https://api.weather.gov/openapi.json#/components/schemas/GridpointHourlyForecastPeriod
type ForecastPeriod struct {
	Number                     int                    `json:"number"`
	Name                       string                 `json:"name"`
	StartTime                  string                 `json:"startTime"`
	EndTime                    string                 `json:"endTime"`
	IsDaytime                  bool                   `json:"isDaytime"`
	Temperature                json.RawMessage        `json:"temperature"`
	TemperatureUnit            string                 `json:"temperatureUnit,omitempty"`
	TemperatureTrend           *string                `json:"temperatureTrend"`
	ProbabilityOfPrecipitation nws.QuantitativeValue  `json:"probabilityOfPrecipitation"`
	Dewpoint                   *nws.QuantitativeValue `json:"dewpoint,omitempty"`
	RelativeHumidity           *nws.QuantitativeValue `json:"relativeHumidity,omitempty"`
	WindSpeed                  json.RawMessage        `json:"windSpeed"`
	WindGust                   json.RawMessage        `json:"windGust,omitempty"`
	WindDirection              string                 `json:"windDirection"`
	Icon                       string                 `json:"icon"`
	ShortForecast              string                 `json:"shortForecast"`
	DetailedForecast           string                 `json:"detailedForecast"`
}

// GetForecast fetches a forecast or forecastHourly link returned by points.Get.
// query accepts NWS parameters such as units=si. Use a caller-owned transport for
// feature-flag headers, or Client.Get for caller-defined period types.
func GetForecast(ctx context.Context, client *nws.Client, resource string, query url.Values) (*nws.Feature[Forecast], error) {
	var result nws.Feature[Forecast]
	if err := client.Get(ctx, resource, query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
