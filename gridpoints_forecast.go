package nws

import (
	"context"
	"encoding/json"
	"net/url"
)

// GridpointForecast models the shared properties of the NWS OpenAPI Gridpoint12hForecast
// and GridpointHourlyForecast schemas. Feature[GridpointForecast] supplies the GeoJSON
// envelope for Gridpoint12hForecastGeoJson or GridpointHourlyForecastGeoJson.
//
// Schemas:
// https://api.weather.gov/openapi.json#/components/schemas/Gridpoint12hForecast
// https://api.weather.gov/openapi.json#/components/schemas/GridpointHourlyForecast
type GridpointForecast struct {
	Units             string                    `json:"units"`
	ForecastGenerator string                    `json:"forecastGenerator"`
	GeneratedAt       string                    `json:"generatedAt"`
	UpdateTime        string                    `json:"updateTime"`
	ValidTimes        string                    `json:"validTimes"`
	Elevation         QuantitativeValue         `json:"elevation"`
	Periods           []GridpointForecastPeriod `json:"periods"`
}

// GridpointForecastPeriod combines the NWS OpenAPI Gridpoint12hForecastPeriod and
// GridpointHourlyForecastPeriod schemas. Dewpoint and RelativeHumidity are
// hourly-only fields. It uses raw JSON for fields whose format depends on NWS
// feature flags: Temperature is an integer or QuantitativeValue; WindSpeed is
// a string or QuantitativeValue; WindGust also permits null. Keeping these
// fields raw leaves callers free to choose a format without converting values
// or units.
//
// Schemas:
// https://api.weather.gov/openapi.json#/components/schemas/Gridpoint12hForecastPeriod
// https://api.weather.gov/openapi.json#/components/schemas/GridpointHourlyForecastPeriod
type GridpointForecastPeriod struct {
	Number                     int                `json:"number"`
	Name                       string             `json:"name"`
	StartTime                  string             `json:"startTime"`
	EndTime                    string             `json:"endTime"`
	IsDaytime                  bool               `json:"isDaytime"`
	Temperature                json.RawMessage    `json:"temperature"`
	TemperatureUnit            string             `json:"temperatureUnit,omitempty"`
	TemperatureTrend           *string            `json:"temperatureTrend"`
	ProbabilityOfPrecipitation QuantitativeValue  `json:"probabilityOfPrecipitation"`
	Dewpoint                   *QuantitativeValue `json:"dewpoint,omitempty"`
	RelativeHumidity           *QuantitativeValue `json:"relativeHumidity,omitempty"`
	WindSpeed                  json.RawMessage    `json:"windSpeed"`
	WindGust                   json.RawMessage    `json:"windGust,omitempty"`
	WindDirection              string             `json:"windDirection"`
	Icon                       string             `json:"icon"`
	ShortForecast              string             `json:"shortForecast"`
	DetailedForecast           string             `json:"detailedForecast"`
}

// GetForecast fetches a forecast or forecastHourly link returned by PointsService.Get.
// query accepts NWS parameters such as units=si. Use a caller-owned transport for
// feature-flag headers, or Client.Get for caller-defined period types.
func (s *GridpointsService) GetForecast(ctx context.Context, resource string, query url.Values) (*Feature[GridpointForecast], error) {
	var result Feature[GridpointForecast]
	if err := s.client.Get(ctx, resource, query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
