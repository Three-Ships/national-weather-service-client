package nws

import (
	"context"
	"encoding/json"
	"fmt"
)

// GridpointsService provides access to NWS gridpoints endpoints.
// Use the corresponding field on a Client constructed with NewClient.
type GridpointsService struct {
	client *Client
}

// GridpointIntervalValue generalizes validTime/value entries in NWS OpenAPI gridpoint
// layers. GridpointQuantitativeValueLayer entries use *float64 for T; weather
// and hazard entries use slices of their categorical values. ValidTime remains
// an ISO 8601 start/duration or start/end string. Callers decide how to expand
// or align the interval.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/GridpointQuantitativeValueLayer/properties/values/items
type GridpointIntervalValue[T any] struct {
	ValidTime string `json:"validTime"`
	Value     T      `json:"value"`
}

// GridpointLayer generalizes the NWS OpenAPI GridpointQuantitativeValueLayer schema and
// the inline weather/hazard layers in Gridpoint. Numeric layers use GridpointLayer[*float64].
// Values retain source order; UOM may be absent for categorical layers.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/GridpointQuantitativeValueLayer
type GridpointLayer[T any] struct {
	UOM    string                      `json:"uom,omitempty"`
	Values []GridpointIntervalValue[T] `json:"values"`
}

// GridpointWeather models an inline weather value in the NWS OpenAPI Gridpoint schema,
// describing one expected weather phenomenon in a grid interval.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/Gridpoint/properties/weather/properties/values/items/properties/value/items
type GridpointWeather struct {
	Coverage   *string           `json:"coverage"`
	Weather    *string           `json:"weather"`
	Intensity  *string           `json:"intensity"`
	Visibility QuantitativeValue `json:"visibility"`
	Attributes []string          `json:"attributes"`
}

// GridpointHazard models an inline hazard value in the NWS OpenAPI Gridpoint schema,
// retaining source P-VTEC codes and a nullable event number.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/Gridpoint/properties/hazards/properties/values/items/properties/value/items
type GridpointHazard struct {
	Phenomenon   string `json:"phenomenon"`
	Significance string `json:"significance"`
	EventNumber  *int   `json:"event_number"`
}

// Gridpoint models the NWS OpenAPI Gridpoint schema used as GridpointGeoJson.properties.
// Numeric Layers are keyed by source names such as "temperature", "skyCover",
// and "quantitativePrecipitation". The schema makes these additional properties,
// so new numeric layers are retained without adding fields. Layers are stored
// in a map here but remain top-level properties in JSON. Missing layers remain
// absent; null values remain nil. Weather and Hazards have distinct categorical
// schemas.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/Gridpoint
type Gridpoint struct {
	ID             string                              `json:"@id,omitempty"`
	Type           string                              `json:"@type,omitempty"`
	Context        json.RawMessage                     `json:"@context,omitempty"`
	Geometry       string                              `json:"geometry,omitempty"`
	UpdateTime     string                              `json:"updateTime"`
	ValidTimes     string                              `json:"validTimes"`
	Elevation      QuantitativeValue                   `json:"elevation"`
	ForecastOffice string                              `json:"forecastOffice"`
	GridID         string                              `json:"gridId"`
	GridX          int                                 `json:"gridX"`
	GridY          int                                 `json:"gridY"`
	Weather        *GridpointLayer[[]GridpointWeather] `json:"weather,omitempty"`
	Hazards        *GridpointLayer[[]GridpointHazard]  `json:"hazards,omitempty"`
	Layers         map[string]GridpointLayer[*float64] `json:"-"`
}

type gridpointFields Gridpoint

// UnmarshalJSON rejects additional properties that cannot be decoded as numeric
// layers.
func (g *Gridpoint) UnmarshalJSON(data []byte) error {
	var fields gridpointFields
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, key := range gridpointKeys {
		delete(properties, key)
	}
	fields.Layers = make(map[string]GridpointLayer[*float64], len(properties))
	for name, raw := range properties {
		var layer GridpointLayer[*float64]
		if err := json.Unmarshal(raw, &layer); err != nil {
			return fmt.Errorf("grid layer %q: %w", name, err)
		}
		fields.Layers[name] = layer
	}
	*g = Gridpoint(fields)
	return nil
}

// MarshalJSON emits Layers as top-level properties to match the NWS JSON format.
// Layer names that conflict with metadata or categorical fields are errors.
func (g Gridpoint) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(gridpointFields(g))
	if err != nil {
		return nil, err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return nil, err
	}
	for name, layer := range g.Layers {
		for _, key := range gridpointKeys {
			if name == key {
				return nil, fmt.Errorf("grid layer %q conflicts with metadata", name)
			}
		}
		raw, err := json.Marshal(layer)
		if err != nil {
			return nil, err
		}
		properties[name] = raw
	}
	return json.Marshal(properties)
}

var gridpointKeys = [...]string{
	"@id", "@type", "@context", "geometry", "updateTime", "validTimes", "elevation",
	"forecastOffice", "gridId", "gridX", "gridY", "weather", "hazards",
}

// Get fetches a discovered forecastGridData URL (or API path).
func (s *GridpointsService) Get(ctx context.Context, resource string) (*Feature[Gridpoint], *Response, error) {
	var result Feature[Gridpoint]
	response, err := s.client.Get(ctx, resource, nil, &result)
	if err != nil {
		return nil, response, err
	}
	return &result, response, nil
}
