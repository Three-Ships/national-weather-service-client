// Package nws provides an unofficial client for the National Weather Service API.
//
// Create a [Client] with [NewClient] and call its resource services.
// Reuse the client across requests; it is safe for concurrent use.
//
// Resource methods return a typed result, a [Response], and an error.
// Results are nil on error. [Client.Get] documents which errors return a response.
//
// See https://www.weather.gov/documentation/services-web-api for API guidance.
package nws
