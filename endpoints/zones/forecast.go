package zones

import (
	"context"
	"encoding/json"
	"net/url"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/internal/endpoint"
)

// Forecast models the NWS OpenAPI ZoneForecast schema used as ZoneForecastGeoJson.properties.
// It is a textual forecast for a zone, not a grid forecast. Periods do not include
// the numeric weather fields or start/end timestamps of grid forecasts.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/ZoneForecast
type Forecast struct {
	Context  json.RawMessage  `json:"@context,omitempty"`
	Geometry string           `json:"geometry,omitempty"`
	Zone     string           `json:"zone"`
	Updated  string           `json:"updated"`
	Periods  []ForecastPeriod `json:"periods"`
}

// ForecastPeriod models an inline periods entry in the NWS OpenAPI ZoneForecast
// schema, keeping the NWS period number, label, and forecast text.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/ZoneForecast/properties/periods/items
type ForecastPeriod struct {
	Number           int    `json:"number"`
	Name             string `json:"name"`
	DetailedForecast string `json:"detailedForecast"`
}

// GetForecast retrieves a zone's current forecast without converting its periods
// into grid forecast periods.
func GetForecast(ctx context.Context, client *nws.Client, zoneType, id string) (*nws.Feature[Forecast], error) {
	if err := endpoint.ValidateSegment(zoneType); err != nil {
		return nil, err
	}
	if err := endpoint.ValidateSegment(id); err != nil {
		return nil, err
	}
	var result nws.Feature[Forecast]
	if err := client.Get(ctx, "zones/"+url.PathEscape(zoneType)+"/"+url.PathEscape(id)+"/forecast", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
