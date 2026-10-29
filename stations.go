package nws

import (
	"context"
	"net/url"
)

// StationsService provides access to NWS stations endpoints.
// Use the corresponding field on a Client constructed with NewClient.
type StationsService struct {
	client *Client
}

// Station models the NWS OpenAPI ObservationStation schema used as
// ObservationStationGeoJson.properties, containing observation-station metadata.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/ObservationStation
type Station struct {
	ID                string             `json:"@id,omitempty"`
	Elevation         QuantitativeValue  `json:"elevation"`
	StationIdentifier string             `json:"stationIdentifier"`
	Name              string             `json:"name"`
	TimeZone          string             `json:"timeZone"`
	Provider          string             `json:"provider"`
	SubProvider       string             `json:"subProvider"`
	Forecast          string             `json:"forecast"`
	County            string             `json:"county"`
	FireWeatherZone   string             `json:"fireWeatherZone"`
	Distance          *QuantitativeValue `json:"distance,omitempty"`
	Bearing           *QuantitativeValue `json:"bearing,omitempty"`
}

// StationCollection models one page of the NWS OpenAPI ObservationStationCollectionGeoJson
// schema. ObservationStations retains source station URLs when present;
// Pagination is not followed automatically.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/ObservationStationCollectionGeoJson
type StationCollection struct {
	FeatureCollection[Station]
	Pagination          *PaginationInfo `json:"pagination,omitempty"`
	ObservationStations []string        `json:"observationStations,omitempty"`
}

// Get fetches station metadata by its NWS identifier.
func (s *StationsService) Get(ctx context.Context, id string) (*Feature[Station], *Response, error) {
	if err := validateResourceID(id); err != nil {
		return nil, nil, err
	}
	var result Feature[Station]
	response, err := s.client.Get(ctx, "stations/"+url.PathEscape(id), nil, &result)
	if err != nil {
		return nil, response, err
	}
	return &result, response, nil
}

// List fetches one station collection page. Resource may be "stations" or a
// discovered observationStations link; query accepts the endpoint's filters.
func (s *StationsService) List(ctx context.Context, resource string, query url.Values) (*StationCollection, *Response, error) {
	var result StationCollection
	response, err := s.client.Get(ctx, resource, query, &result)
	if err != nil {
		return nil, response, err
	}
	return &result, response, nil
}
