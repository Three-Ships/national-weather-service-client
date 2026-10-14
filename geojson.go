package nws

import "encoding/json"

// Feature models the NWS OpenAPI GeoJsonFeature schema with properties of type T.
// Geometry and Context retain their source representation because NWS serves
// multiple geometry and JSON-LD context shapes.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/GeoJsonFeature
type Feature[T any] struct {
	Context    json.RawMessage `json:"@context,omitempty"`
	ID         string          `json:"id,omitempty"`
	Type       string          `json:"type"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties T               `json:"properties"`
}

// FeatureCollection models one page of the NWS OpenAPI GeoJsonFeatureCollection
// schema. Resource packages add collection-specific metadata, such as alert
// timestamps and station pagination.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/GeoJsonFeatureCollection
type FeatureCollection[T any] struct {
	Context  json.RawMessage `json:"@context,omitempty"`
	Type     string          `json:"type"`
	Features []Feature[T]    `json:"features"`
}

// PaginationInfo models the NWS OpenAPI PaginationInfo schema. Its next-page URL
// can be passed to Client.Get.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/PaginationInfo
type PaginationInfo struct {
	Next string `json:"next"`
}
