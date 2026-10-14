// Package stations retrieves NWS station metadata and weather observations.
package stations

import (
	"context"
	"net/url"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/internal/endpoint"
)

// Station models the NWS OpenAPI ObservationStation schema used as
// ObservationStationGeoJson.properties, containing observation-station metadata.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/ObservationStation
type Station struct {
	ID                string                 `json:"@id,omitempty"`
	Elevation         nws.QuantitativeValue  `json:"elevation"`
	StationIdentifier string                 `json:"stationIdentifier"`
	Name              string                 `json:"name"`
	TimeZone          string                 `json:"timeZone"`
	Provider          string                 `json:"provider"`
	SubProvider       string                 `json:"subProvider"`
	Forecast          string                 `json:"forecast"`
	County            string                 `json:"county"`
	FireWeatherZone   string                 `json:"fireWeatherZone"`
	Distance          *nws.QuantitativeValue `json:"distance,omitempty"`
	Bearing           *nws.QuantitativeValue `json:"bearing,omitempty"`
}

// Collection models one page of the NWS OpenAPI ObservationStationCollectionGeoJson
// schema. ObservationStations retains source station URLs when present;
// Pagination is not followed automatically.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/ObservationStationCollectionGeoJson
type Collection struct {
	nws.FeatureCollection[Station]
	Pagination          *nws.PaginationInfo `json:"pagination,omitempty"`
	ObservationStations []string            `json:"observationStations,omitempty"`
}

// Get fetches station metadata by its NWS identifier.
func Get(ctx context.Context, client *nws.Client, id string) (*nws.Feature[Station], error) {
	if err := endpoint.ValidateSegment(id); err != nil {
		return nil, err
	}
	var result nws.Feature[Station]
	if err := client.Get(ctx, "stations/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// List fetches one station collection page. Resource may be "stations" or a
// discovered observationStations link; query accepts the endpoint's filters.
func List(ctx context.Context, client *nws.Client, resource string, query url.Values) (*Collection, error) {
	var result Collection
	if err := client.Get(ctx, resource, query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
