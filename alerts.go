package nws

import (
	"context"
	"encoding/json"
	"net/url"
)

// AlertsService provides access to NWS alerts endpoints.
// Use the corresponding field on a Client constructed with NewClient.
type AlertsService struct {
	client *Client
}

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

// AlertCollection models the NWS OpenAPI AlertCollectionGeoJson and
// ActiveAlertCollectionGeoJson schemas. Pagination is optional, belongs to
// AlertCollectionGeoJson, and is not followed automatically.
//
// Schemas:
// https://api.weather.gov/openapi.json#/components/schemas/AlertCollectionGeoJson
// https://api.weather.gov/openapi.json#/components/schemas/ActiveAlertCollectionGeoJson
type AlertCollection struct {
	FeatureCollection[Alert]
	Title      string          `json:"title,omitempty"`
	Updated    string          `json:"updated,omitempty"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// ActiveAlertCounts models the inline success-response schema for GET /alerts/active/count,
// grouping active alerts by land/marine coverage and NWS codes. Missing map keys
// mean NWS supplied no count for that code.
//
// Schema: https://api.weather.gov/openapi.json#/paths/~1alerts~1active~1count/get/responses/200/content/application~1ld+json/schema
type ActiveAlertCounts struct {
	Total   int            `json:"total"`
	Land    int            `json:"land"`
	Marine  int            `json:"marine"`
	Regions map[string]int `json:"regions"`
	Areas   map[string]int `json:"areas"`
	Zones   map[string]int `json:"zones"`
}

// AlertEventTypes models the inline success-response schema for GET /alerts/types,
// containing NWS's recognized alert event names.
//
// Schema: https://api.weather.gov/openapi.json#/paths/~1alerts~1types/get/responses/200/content/application~1ld+json/schema
type AlertEventTypes struct {
	Types []string `json:"eventTypes"`
}

// CountActive returns the API's unfiltered active-alert counts.
func (s *AlertsService) CountActive(ctx context.Context) (*ActiveAlertCounts, error) {
	var result ActiveAlertCounts
	if err := s.client.Get(ctx, "alerts/active/count", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTypes returns recognized event names without treating the list as a fixed
// enum enforced by the client.
func (s *AlertsService) ListTypes(ctx context.Context) (*AlertEventTypes, error) {
	var result AlertEventTypes
	if err := s.client.Get(ctx, "alerts/types", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Get fetches an alert by its NWS identifier, not its API URL.
func (s *AlertsService) Get(ctx context.Context, id string) (*Feature[Alert], error) {
	if err := validateResourceID(id); err != nil {
		return nil, err
	}
	var result Feature[Alert]
	if err := s.client.Get(ctx, "alerts/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// List returns one page of alerts with NWS query parameters (including cursor).
// Follow AlertCollection.Pagination.Next with Client.Get into an AlertCollection.
func (s *AlertsService) List(ctx context.Context, query url.Values) (*AlertCollection, error) {
	var result AlertCollection
	if err := s.client.Get(ctx, "alerts", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListActive returns currently active alerts with caller-selected filters.
// No status filter is imposed; pass status=actual to exclude exercises/tests.
func (s *AlertsService) ListActive(ctx context.Context, query url.Values) (*AlertCollection, error) {
	var result AlertCollection
	if err := s.client.Get(ctx, "alerts/active", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
