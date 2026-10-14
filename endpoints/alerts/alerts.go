// Package alerts retrieves NWS alerts without application-specific filtering.
package alerts

import (
	"context"
	"encoding/json"
	"net/url"

	nws "github.com/Three-Ships/national-weather-service-client"
	"github.com/Three-Ships/national-weather-service-client/internal/endpoint"
)

// Alert models the NWS OpenAPI Alert schema used as AlertGeoJson.properties.
// It keeps NWS CAP-derived fields, with enums and timestamps left as strings.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/Alert
type Alert struct {
	ID            string                       `json:"id"`
	AreaDesc      string                       `json:"areaDesc"`
	Geocode       *AlertGeocode                `json:"geocode"`
	AffectedZones []string                     `json:"affectedZones"`
	References    []AlertReference             `json:"references"`
	Sent          string                       `json:"sent"`
	Effective     string                       `json:"effective"`
	Onset         *string                      `json:"onset"`
	Expires       string                       `json:"expires"`
	Ends          *string                      `json:"ends"`
	Status        string                       `json:"status"`
	MessageType   string                       `json:"messageType"`
	Category      string                       `json:"category"`
	Severity      string                       `json:"severity"`
	Certainty     string                       `json:"certainty"`
	Urgency       string                       `json:"urgency"`
	Event         string                       `json:"event"`
	Sender        string                       `json:"sender"`
	SenderName    string                       `json:"senderName"`
	Headline      *string                      `json:"headline"`
	Description   string                       `json:"description"`
	Instruction   *string                      `json:"instruction"`
	Note          *string                      `json:"note"`
	Response      string                       `json:"response"`
	Parameters    map[string][]json.RawMessage `json:"parameters"`
	Scope         string                       `json:"scope"`
	Code          string                       `json:"code"`
	Language      string                       `json:"language"`
	Web           string                       `json:"web"`
	EventCode     map[string][]json.RawMessage `json:"eventCode"`
}

// AlertGeocode models the inline geocode object in the NWS OpenAPI Alert schema,
// listing NWS public zone/county identifiers and SAME codes.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/Alert/properties/geocode
type AlertGeocode struct {
	UGC  []string `json:"UGC"`
	SAME []string `json:"SAME"`
}

// AlertReference models an inline references entry in the NWS OpenAPI Alert
// schema, identifying a prior alert that this alert updates or replaces.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/Alert/properties/references/items
type AlertReference struct {
	ID         string `json:"@id"`
	Identifier string `json:"identifier"`
	Sender     string `json:"sender"`
	Sent       string `json:"sent"`
}

// Collection models the NWS OpenAPI AlertCollectionGeoJson and
// ActiveAlertCollectionGeoJson schemas. Pagination is optional, belongs to
// AlertCollectionGeoJson, and is not followed automatically.
//
// Schemas:
// https://api.weather.gov/openapi.json#/components/schemas/AlertCollectionGeoJson
// https://api.weather.gov/openapi.json#/components/schemas/ActiveAlertCollectionGeoJson
type Collection struct {
	nws.FeatureCollection[Alert]
	Title      string              `json:"title,omitempty"`
	Updated    string              `json:"updated,omitempty"`
	Pagination *nws.PaginationInfo `json:"pagination,omitempty"`
}

// ActiveCounts models the inline success-response schema for GET /alerts/active/count,
// grouping active alerts by land/marine coverage and NWS codes. Missing map keys
// mean NWS supplied no count for that code.
//
// Schema: https://api.weather.gov/openapi.json#/paths/~1alerts~1active~1count/get/responses/200/content/application~1ld+json/schema
type ActiveCounts struct {
	Total   int            `json:"total"`
	Land    int            `json:"land"`
	Marine  int            `json:"marine"`
	Regions map[string]int `json:"regions"`
	Areas   map[string]int `json:"areas"`
	Zones   map[string]int `json:"zones"`
}

// EventTypes models the inline success-response schema for GET /alerts/types,
// containing NWS's recognized alert event names.
//
// Schema: https://api.weather.gov/openapi.json#/paths/~1alerts~1types/get/responses/200/content/application~1ld+json/schema
type EventTypes struct {
	Types []string `json:"eventTypes"`
}

// CountActive returns the API's unfiltered active-alert counts.
func CountActive(ctx context.Context, client *nws.Client) (*ActiveCounts, error) {
	var result ActiveCounts
	if err := client.Get(ctx, "alerts/active/count", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTypes returns recognized event names without treating the list as a fixed
// enum enforced by the client.
func ListTypes(ctx context.Context, client *nws.Client) (*EventTypes, error) {
	var result EventTypes
	if err := client.Get(ctx, "alerts/types", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Get fetches an alert by its NWS identifier, not its API URL.
func Get(ctx context.Context, client *nws.Client, id string) (*nws.Feature[Alert], error) {
	if err := endpoint.ValidateSegment(id); err != nil {
		return nil, err
	}
	var result nws.Feature[Alert]
	if err := client.Get(ctx, "alerts/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// List returns one page of alerts with NWS query parameters (including cursor).
// Follow Collection.Pagination.Next with Client.Get into a Collection.
func List(ctx context.Context, client *nws.Client, query url.Values) (*Collection, error) {
	var result Collection
	if err := client.Get(ctx, "alerts", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListActive returns currently active alerts with caller-selected filters.
// No status filter is imposed; pass status=actual to exclude exercises/tests.
func ListActive(ctx context.Context, client *nws.Client, query url.Values) (*Collection, error) {
	var result Collection
	if err := client.Get(ctx, "alerts/active", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
