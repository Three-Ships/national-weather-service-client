package nws

// QuantitativeValue models the NWS OpenAPI QuantitativeValue measurement schema.
// It keeps NWS units and optional bounds. A nil Value means missing data; a non-nil
// pointer to zero is a real measurement. MinValue and MaxValue come from NWS and
// are never calculated here.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/QuantitativeValue
type QuantitativeValue struct {
	Value          *float64 `json:"value"`
	MaxValue       *float64 `json:"maxValue,omitempty"`
	MinValue       *float64 `json:"minValue,omitempty"`
	UnitCode       string   `json:"unitCode"`
	QualityControl string   `json:"qualityControl,omitempty"`
}
