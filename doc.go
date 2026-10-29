// Package nws provides an unofficial client for the National Weather Service API.
//
// Construct a Client with NewClient, then use its Points, Gridpoints, Alerts,
// Stations, and Zones services. All services share the client's HTTP policy and
// pacing. Resource operations and source types are defined in this package.
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
