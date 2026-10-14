package stations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/internal/endpoint"
)

// Observation models the NWS OpenAPI Observation schema used as
// ObservationGeoJson.properties. It keeps NWS measurements, unit codes,
// quality-control flags, and timestamps unchanged. A measurement's nil Value
// means the reading is missing.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/Observation
type Observation struct {
	Context                   json.RawMessage       `json:"@context,omitempty"`
	Geometry                  string                `json:"geometry,omitempty"`
	ID                        string                `json:"@id,omitempty"`
	Type                      string                `json:"@type,omitempty"`
	Elevation                 nws.QuantitativeValue `json:"elevation"`
	Station                   string                `json:"station"`
	StationID                 string                `json:"stationId"`
	StationName               string                `json:"stationName"`
	Timestamp                 string                `json:"timestamp"`
	RawMessage                string                `json:"rawMessage"`
	TextDescription           string                `json:"textDescription"`
	Icon                      *string               `json:"icon"`
	PresentWeather            []MetarPhenomenon     `json:"presentWeather"`
	Temperature               nws.QuantitativeValue `json:"temperature"`
	Dewpoint                  nws.QuantitativeValue `json:"dewpoint"`
	WindDirection             nws.QuantitativeValue `json:"windDirection"`
	WindSpeed                 nws.QuantitativeValue `json:"windSpeed"`
	WindGust                  nws.QuantitativeValue `json:"windGust"`
	BarometricPressure        nws.QuantitativeValue `json:"barometricPressure"`
	SeaLevelPressure          nws.QuantitativeValue `json:"seaLevelPressure"`
	Visibility                nws.QuantitativeValue `json:"visibility"`
	MaxTemperatureLast24Hours nws.QuantitativeValue `json:"maxTemperatureLast24Hours"`
	MinTemperatureLast24Hours nws.QuantitativeValue `json:"minTemperatureLast24Hours"`
	PrecipitationLastHour     nws.QuantitativeValue `json:"precipitationLastHour"`
	PrecipitationLast3Hours   nws.QuantitativeValue `json:"precipitationLast3Hours"`
	PrecipitationLast6Hours   nws.QuantitativeValue `json:"precipitationLast6Hours"`
	RelativeHumidity          nws.QuantitativeValue `json:"relativeHumidity"`
	WindChill                 nws.QuantitativeValue `json:"windChill"`
	HeatIndex                 nws.QuantitativeValue `json:"heatIndex"`
	CloudLayers               []CloudLayer          `json:"cloudLayers"`
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
	Base   nws.QuantitativeValue `json:"base"`
	Amount string                `json:"amount"`
}

// ObservationCollection models one page of the NWS OpenAPI ObservationCollectionGeoJson
// schema. Pagination is optional and is not followed automatically.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/ObservationCollectionGeoJson
type ObservationCollection struct {
	nws.FeatureCollection[Observation]
	Pagination *nws.PaginationInfo `json:"pagination,omitempty"`
}

// ListObservations returns station history with NWS query parameters such as
// start, end, limit, and cursor. No time window or quality filter is imposed.
func ListObservations(ctx context.Context, client *nws.Client, stationID string, query url.Values) (*ObservationCollection, error) {
	if err := endpoint.ValidateSegment(stationID); err != nil {
		return nil, err
	}
	var result ObservationCollection
	if err := client.Get(ctx, "stations/"+url.PathEscape(stationID)+"/observations", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetLatestObservation accepts the NWS require_qc query parameter. It does not
// impose a quality-control requirement on callers.
func GetLatestObservation(ctx context.Context, client *nws.Client, stationID string, query url.Values) (*nws.Feature[Observation], error) {
	if err := endpoint.ValidateSegment(stationID); err != nil {
		return nil, err
	}
	var result nws.Feature[Observation]
	if err := client.Get(ctx, "stations/"+url.PathEscape(stationID)+"/observations/latest", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetObservation requires an RFC3339 timestamp. Its original offset and precision
// are retained in the request; invalid timestamps fail before HTTP.
func GetObservation(ctx context.Context, client *nws.Client, stationID, timestamp string) (*nws.Feature[Observation], error) {
	if err := endpoint.ValidateSegment(stationID); err != nil {
		return nil, err
	}
	if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
		return nil, fmt.Errorf("nws: invalid observation timestamp: %w", err)
	}
	var result nws.Feature[Observation]
	if err := client.Get(ctx, "stations/"+url.PathEscape(stationID)+"/observations/"+url.PathEscape(timestamp), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
