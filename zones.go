package nws

import (
	"context"
	"net/url"
)

// ZonesService provides access to NWS zones endpoints.
// Use the corresponding field on a Client constructed with NewClient.
type ZonesService struct {
	client *Client
}

// Zone models the NWS OpenAPI Zone schema used as ZoneGeoJson.properties.
// It describes a forecast, county, fire, or marine zone.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/Zone
type Zone struct {
	ID                      string   `json:"id"`
	Type                    string   `json:"type"`
	Name                    string   `json:"name"`
	EffectiveDate           string   `json:"effectiveDate"`
	ExpirationDate          string   `json:"expirationDate"`
	State                   *string  `json:"state"`
	ForecastOffice          string   `json:"forecastOffice"`
	GridIdentifier          string   `json:"gridIdentifier"`
	AWIPSLocationIdentifier string   `json:"awipsLocationIdentifier"`
	CWA                     []string `json:"cwa"`
	ForecastOffices         []string `json:"forecastOffices"`
	TimeZone                []string `json:"timeZone"`
	ObservationStations     []string `json:"observationStations"`
	RadarStation            *string  `json:"radarStation"`
}

// List returns zones matching NWS query parameters such as area, type, point,
// and include_geometry. Multi-value filters use comma-separated strings.
func (s *ZonesService) List(ctx context.Context, query url.Values) (*FeatureCollection[Zone], *Response, error) {
	var result FeatureCollection[Zone]
	response, err := s.client.Get(ctx, "zones", query, &result)
	if err != nil {
		return nil, response, err
	}
	return &result, response, nil
}

// ListByType selects the zone type in the route; query retains the list filters.
func (s *ZonesService) ListByType(ctx context.Context, zoneType string, query url.Values) (*FeatureCollection[Zone], *Response, error) {
	if err := validateResourceID(zoneType); err != nil {
		return nil, nil, err
	}
	var result FeatureCollection[Zone]
	response, err := s.client.Get(ctx, "zones/"+url.PathEscape(zoneType), query, &result)
	if err != nil {
		return nil, response, err
	}
	return &result, response, nil
}

// Get fetches metadata by NWS zone type and identifier. To use a discovered
// forecastZone URL directly, call Client.Get with an Feature[Zone].
func (s *ZonesService) Get(ctx context.Context, zoneType, id string) (*Feature[Zone], *Response, error) {
	if err := validateResourceID(zoneType); err != nil {
		return nil, nil, err
	}
	if err := validateResourceID(id); err != nil {
		return nil, nil, err
	}
	var result Feature[Zone]
	response, err := s.client.Get(ctx, "zones/"+url.PathEscape(zoneType)+"/"+url.PathEscape(id), nil, &result)
	if err != nil {
		return nil, response, err
	}
	return &result, response, nil
}
