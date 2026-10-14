// Package zones retrieves NWS forecast, county, fire, and marine zone metadata.
package zones

import (
	"context"
	"net/url"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/internal/endpoint"
)

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
func List(ctx context.Context, client *nws.Client, query url.Values) (*nws.FeatureCollection[Zone], error) {
	var result nws.FeatureCollection[Zone]
	if err := client.Get(ctx, "zones", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListByType selects the zone type in the route; query retains the list filters.
func ListByType(ctx context.Context, client *nws.Client, zoneType string, query url.Values) (*nws.FeatureCollection[Zone], error) {
	if err := endpoint.ValidateSegment(zoneType); err != nil {
		return nil, err
	}
	var result nws.FeatureCollection[Zone]
	if err := client.Get(ctx, "zones/"+url.PathEscape(zoneType), query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Get fetches metadata by NWS zone type and identifier. To use a discovered
// forecastZone URL directly, call Client.Get with an nws.Feature[Zone].
func Get(ctx context.Context, client *nws.Client, zoneType, id string) (*nws.Feature[Zone], error) {
	if err := endpoint.ValidateSegment(zoneType); err != nil {
		return nil, err
	}
	if err := endpoint.ValidateSegment(id); err != nil {
		return nil, err
	}
	var result nws.Feature[Zone]
	if err := client.Get(ctx, "zones/"+url.PathEscape(zoneType)+"/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
