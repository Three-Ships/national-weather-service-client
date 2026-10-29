package nws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// Observation models the NWS OpenAPI Observation schema used as
// ObservationGeoJson.properties. It keeps NWS measurements, unit codes,
// quality-control flags, and timestamps unchanged. A measurement's nil Value
// means the reading is missing.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/Observation
type Observation struct {
	Context                   json.RawMessage   `json:"@context,omitempty"`
	Geometry                  string            `json:"geometry,omitempty"`
	ID                        string            `json:"@id,omitempty"`
	Type                      string            `json:"@type,omitempty"`
	Elevation                 QuantitativeValue `json:"elevation"`
	Station                   string            `json:"station"`
	StationID                 string            `json:"stationId"`
	StationName               string            `json:"stationName"`
	Timestamp                 string            `json:"timestamp"`
	RawMessage                string            `json:"rawMessage"`
	TextDescription           string            `json:"textDescription"`
	Icon                      *string           `json:"icon"`
	PresentWeather            []MetarPhenomenon `json:"presentWeather"`
	Temperature               QuantitativeValue `json:"temperature"`
	Dewpoint                  QuantitativeValue `json:"dewpoint"`
	WindDirection             QuantitativeValue `json:"windDirection"`
	WindSpeed                 QuantitativeValue `json:"windSpeed"`
	WindGust                  QuantitativeValue `json:"windGust"`
	BarometricPressure        QuantitativeValue `json:"barometricPressure"`
	SeaLevelPressure          QuantitativeValue `json:"seaLevelPressure"`
	Visibility                QuantitativeValue `json:"visibility"`
	MaxTemperatureLast24Hours QuantitativeValue `json:"maxTemperatureLast24Hours"`
	MinTemperatureLast24Hours QuantitativeValue `json:"minTemperatureLast24Hours"`
	PrecipitationLastHour     QuantitativeValue `json:"precipitationLastHour"`
	PrecipitationLast3Hours   QuantitativeValue `json:"precipitationLast3Hours"`
	PrecipitationLast6Hours   QuantitativeValue `json:"precipitationLast6Hours"`
	RelativeHumidity          QuantitativeValue `json:"relativeHumidity"`
	WindChill                 QuantitativeValue `json:"windChill"`
	HeatIndex                 QuantitativeValue `json:"heatIndex"`
	CloudLayers               []CloudLayer      `json:"cloudLayers"`
}

// MetarPhenomenon models the NWS OpenAPI MetarPhenomenon schema, retaining decoded
// METAR codes and raw text. Intensity and Modifier may be null; enums remain
// strings so new NWS codes are not discarded.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/MetarPhenomenon
type MetarPhenomenon struct {
	Intensity  *string `json:"intensity"`
	Modifier   *string `json:"modifier"`
	Weather    string  `json:"weather"`
	RawString  string  `json:"rawString"`
	InVicinity bool    `json:"inVicinity"`
}

// CloudLayer models an inline cloudLayers entry in the NWS OpenAPI Observation
// schema, keeping NWS base-height units and the METAR sky-coverage code.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/Observation/properties/cloudLayers/items
type CloudLayer struct {
	Base   QuantitativeValue `json:"base"`
	Amount string            `json:"amount"`
}

// ObservationCollection models one page of the NWS OpenAPI ObservationCollectionGeoJson
// schema. Pagination is optional and is not followed automatically.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/ObservationCollectionGeoJson
type ObservationCollection struct {
	FeatureCollection[Observation]
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// ListObservations returns station history with NWS query parameters such as
// start, end, limit, and cursor. No time window or quality filter is imposed.
func (s *StationsService) ListObservations(ctx context.Context, stationID string, query url.Values) (*ObservationCollection, *Response, error) {
	if err := validateResourceID(stationID); err != nil {
		return nil, nil, err
	}
	var result ObservationCollection
	response, err := s.client.Get(ctx, "stations/"+url.PathEscape(stationID)+"/observations", query, &result)
	if err != nil {
		return nil, response, err
	}
	return &result, response, nil
}

// GetLatestObservation accepts the NWS require_qc query parameter. It does not
// impose a quality-control requirement on callers.
func (s *StationsService) GetLatestObservation(ctx context.Context, stationID string, query url.Values) (*Feature[Observation], *Response, error) {
	if err := validateResourceID(stationID); err != nil {
		return nil, nil, err
	}
	var result Feature[Observation]
	response, err := s.client.Get(ctx, "stations/"+url.PathEscape(stationID)+"/observations/latest", query, &result)
	if err != nil {
		return nil, response, err
	}
	return &result, response, nil
}

// GetObservation requires an RFC3339 timestamp. Its original offset and precision
// are retained in the request; invalid timestamps fail before HTTP.
func (s *StationsService) GetObservation(ctx context.Context, stationID, timestamp string) (*Feature[Observation], *Response, error) {
	if err := validateResourceID(stationID); err != nil {
		return nil, nil, err
	}
	if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
		return nil, nil, fmt.Errorf("nws: invalid observation timestamp: %w", err)
	}
	var result Feature[Observation]
	response, err := s.client.Get(ctx, "stations/"+url.PathEscape(stationID)+"/observations/"+url.PathEscape(timestamp), nil, &result)
	if err != nil {
		return nil, response, err
	}
	return &result, response, nil
}
