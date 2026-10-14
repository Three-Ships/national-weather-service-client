// Package nws provides an unofficial client for the National Weather Service API.
//
// Clients are safe for concurrent use. Each operation is synchronous and owns
// its retries; callers own batch concurrency, overall deadlines, and logging.
// Reuse the same Client to share connection pooling and pacing.
// See https://www.weather.gov/documentation/services-web-api for API guidance.
package nws
