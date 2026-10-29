package nws

import (
	"context"
	"errors"
	"fmt"
	"math"
)

// PointsService provides access to NWS points endpoints.
// Use the corresponding field on a Client constructed with NewClient.
type PointsService struct {
	client *Client
}

// Get resolves a coordinate to NWS metadata and resource links. Coordinates are
// rounded to four decimal places as expected by NWS. Non-finite or out-of-range
// inputs fail before any HTTP request; linked resources require separate calls.
func (s *PointsService) Get(ctx context.Context, latitude, longitude float64) (*Feature[Point], *Response, error) {
	if math.IsNaN(latitude) || math.IsNaN(longitude) || math.IsInf(latitude, 0) || math.IsInf(longitude, 0) ||
		latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return nil, nil, errors.New("nws: coordinates must be finite, latitude in [-90,90], longitude in [-180,180]")
	}
	var result Feature[Point]
	response, err := s.client.Get(ctx, fmt.Sprintf("points/%.4f,%.4f", latitude, longitude), nil, &result)
	if err != nil {
		return nil, response, err
	}
	return &result, response, nil
}

// Point models the NWS OpenAPI Point schema used as PointGeoJson.properties.
// ForecastZone, County, FireWeatherZone, and ForecastOffice are URLs to fetch
// separately. AstronomicalData applies to the current day reported by NWS.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/Point
type Point struct {
	ID                  string                     `json:"@id,omitempty"`
	Type                string                     `json:"type"`
	CWA                 string                     `json:"cwa"`
	ForecastOffice      string                     `json:"forecastOffice"`
	GridID              string                     `json:"gridId"`
	GridX               int                        `json:"gridX"`
	GridY               int                        `json:"gridY"`
	Forecast            string                     `json:"forecast"`
	ForecastHourly      string                     `json:"forecastHourly"`
	ForecastGridData    string                     `json:"forecastGridData"`
	ObservationStations string                     `json:"observationStations"`
	RelativeLocation    *Feature[RelativeLocation] `json:"relativeLocation,omitempty"`
	ForecastZone        string                     `json:"forecastZone"`
	County              string                     `json:"county"`
	FireWeatherZone     string                     `json:"fireWeatherZone"`
	TimeZone            string                     `json:"timeZone"`
	RadarStation        string                     `json:"radarStation"`
	AstronomicalData    *AstronomicalData          `json:"astronomicalData,omitempty"`
	NWR                 *WeatherRadio              `json:"nwr,omitempty"`
}

// RelativeLocation models the NWS OpenAPI RelativeLocation schema used as
// RelativeLocationGeoJson.properties. It describes a nearby named location.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/RelativeLocation
type RelativeLocation struct {
	City     string            `json:"city"`
	State    string            `json:"state"`
	Distance QuantitativeValue `json:"distance"`
	Bearing  QuantitativeValue `json:"bearing"`
}

// WeatherRadio models the inline nwr object in the NWS OpenAPI Point schema,
// containing NOAA Weather Radio discovery metadata.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/Point/properties/nwr
type WeatherRadio struct {
	Transmitter    *string `json:"transmitter"`
	SAMECode       string  `json:"sameCode"`
	AreaBroadcast  *string `json:"areaBroadcast"`
	PointBroadcast *string `json:"pointBroadcast"`
}
