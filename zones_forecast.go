package nws

import (
	"context"
	"encoding/json"
	"net/url"
)

// ZoneForecast models the NWS OpenAPI ZoneForecast schema used as ZoneForecastGeoJson.properties.
// It is a textual forecast for a zone, not a grid forecast. Periods do not include
// the numeric weather fields or start/end timestamps of grid forecasts.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/ZoneForecast
type ZoneForecast struct {
	Context  json.RawMessage      `json:"@context,omitempty"`
	Geometry string               `json:"geometry,omitempty"`
	Zone     string               `json:"zone"`
	Updated  string               `json:"updated"`
	Periods  []ZoneForecastPeriod `json:"periods"`
}

// ZoneForecastPeriod models an inline periods entry in the NWS OpenAPI ZoneForecast
// schema, keeping the NWS period number, label, and forecast text.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/ZoneForecast/properties/periods/items
type ZoneForecastPeriod struct {
	Number           int    `json:"number"`
	Name             string `json:"name"`
	DetailedForecast string `json:"detailedForecast"`
}

// GetForecast retrieves a zone's current forecast without converting its periods
// into grid forecast periods.
func (s *ZonesService) GetForecast(ctx context.Context, zoneType, id string) (*Feature[ZoneForecast], error) {
	if err := validateResourceID(zoneType); err != nil {
		return nil, err
	}
	if err := validateResourceID(id); err != nil {
		return nil, err
	}
	var result Feature[ZoneForecast]
	if err := s.client.Get(ctx, "zones/"+url.PathEscape(zoneType)+"/"+url.PathEscape(id)+"/forecast", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
