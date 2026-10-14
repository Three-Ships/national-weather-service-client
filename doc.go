// Package nws provides an unofficial client for the National Weather Service API.
//
// Resource operations and source types live in endpoints/points,
// endpoints/gridpoints, endpoints/alerts, and endpoints/stations.
// Pass the same Client to those operations to share HTTP policy and pacing.
//
// Response types reference the NWS OpenAPI schemas they model. Endpoint structs
// generally represent feature properties; Feature supplies the GeoJSON envelope.
// These types decode responses but do not validate all schema constraints.
//
// Clients are safe for concurrent use. Each operation is synchronous and owns
// its retries; callers own batch concurrency, overall deadlines, and logging.
// Source timestamps, units, intervals, and nullable values are not normalized.
// See https://www.weather.gov/documentation/services-web-api for API guidance.
package nws
