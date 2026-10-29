package nws

// AstronomicalData models the NWS OpenAPI AstronomicalData schema, keeping event
// times as RFC3339 strings. Events may be null during polar day or night. The
// client neither calculates times nor converts time zones.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/AstronomicalData
type AstronomicalData struct {
	Sunrise                   *string `json:"sunrise"`
	Sunset                    *string `json:"sunset"`
	Transit                   *string `json:"transit"`
	CivilTwilightBegin        *string `json:"civilTwilightBegin"`
	CivilTwilightEnd          *string `json:"civilTwilightEnd"`
	NauticalTwilightBegin     *string `json:"nauticalTwilightBegin"`
	NauticalTwilightEnd       *string `json:"nauticalTwilightEnd"`
	AstronomicalTwilightBegin *string `json:"astronomicalTwilightBegin"`
	AstronomicalTwilightEnd   *string `json:"astronomicalTwilightEnd"`
}
