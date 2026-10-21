# National Weather Service Go client

An unofficial Go client for the [National Weather Service API](https://api.weather.gov).
Requires Go 1.24 or later.

## Installation

```sh
go get github.com/Three-Ships/national-weather-service-client
```

## Usage

Fetch a forecast for a coordinate inside an application function that returns an error:

```go
import (
    "context"
    "errors"
    "fmt"
    "time"

    nws "github.com/Three-Ships/national-weather-service-client"
    "github.com/Three-Ships/national-weather-service-client/endpoints/gridpoints"
    "github.com/Three-Ships/national-weather-service-client/endpoints/points"
)

client, err := nws.NewClient("example-company-weather/1.0 (weather-ops@example.com)")
if err != nil {
    return err
}

ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
defer cancel()

point, err := points.Get(ctx, client, 38.8977, -77.0365)
if err != nil {
    return err
}
if point.Properties.Forecast == "" {
    return errors.New("point has no forecast URL")
}

// Follow the NWS link because grid assignments can change.
forecast, err := gridpoints.GetForecast(ctx, client, point.Properties.Forecast, nil)
if err != nil {
    return err
}
for _, period := range forecast.Properties.Periods {
    fmt.Println(period.Name, period.StartTime, period.ShortForecast)
}
```

`NewClient` takes your application's HTTP `User-Agent`.
[NWS requires application identification](https://www.weather.gov/documentation/services-web-api)

The client retries transport failures, incomplete successful-response reads,
429, and 5xx responses, with a default limit of four attempts. Each attempt is
one `http.Client.Do` call and follows at most three validated redirects.
Exponential backoff uses jitter and honors `Retry-After` on retryable responses;
the caller's context deadline also bounds retry waits.

Resource URLs and redirects must remain within the configured API origin and
path prefix. Query parameters use `url.Values`; malformed encoding in a resource
URL is rejected.

### Configuration

Pass options to `NewClient` to override the defaults:

- `WithHTTPClient`: custom HTTP client; the default timeout is 30 seconds per attempt.
- `WithRetryPolicy`: attempt budget and backoff; defaults to four total attempts with jitter.
- `WithRequestInterval`: request spacing per client; defaults to 200 milliseconds.
- `WithBaseURL`: API origin or test server; defaults to `https://api.weather.gov`.

Set a context deadline to bound the whole operation, including pacing and retry
waits. Pacing does not coordinate separate clients or processes.

### Query parameters

Pass query parameters as unescaped strings in `url.Values`; the client encodes
them. NWS multi-value filters generally use comma-separated values, for example
`url.Values{"area": {"DC,MD"}}`, rather than repeated keys. The client passes
parameter names and values through to NWS, including parameters added after this
library was released. Malformed encoding in a resource URL, such as `area=%ZZ`,
returns an error rather than silently dropping a filter.

### Working with source values

The client preserves NWS timestamps, ISO 8601 intervals, unit codes, and nullable
measurements. A nil `QuantitativeValue.Value` is missing data, not zero. Parse
timestamps and convert units in your application when needed.

Forecast temperature and wind fields use `json.RawMessage` because NWS can return
scalar values or quantitative-value objects depending on feature flags. The
[forecast decoding example](endpoints/gridpoints/example_test.go) handles both
formats and distinguishes missing data from zero without converting units.
If your application requests a fixed format, use `Client.Get` with your own
response type. Feature-flag headers can be added by a caller-owned transport.

### Batch workloads

Reuse a client for connection pooling and pacing. The default 200ms interval
admits about five HTTP requests per second per client; retries and redirects
consume those admissions too. Pacing does not cap in-flight requests or coordinate
separate clients or processes. Bound worker concurrency in your application.

Successful responses are buffered in full before decoding, with no size limit;
decoded data also consumes memory. Process collection pages
incrementally rather than collecting every page in memory. The client does not
cache responses or deduplicate simultaneous requests; applications with repeated
lookups can add caching through a transport or their own data layer, honoring
NWS cache headers.

### Errors

Request failures return a `*nws.RequestError`. Use `errors.As` to inspect the HTTP
status and NWS problem type:

```go
var failure *nws.RequestError
if errors.As(err, &failure) {
    fmt.Println(failure.StatusCode, failure.Problem.Type)
}
```

The client handles retries. `Retryable` describes the failure, not whether
attempts remain. Configuration and input validation errors return before HTTP.
Use `errors.Is` to detect context cancellation or deadlines. Avoid adding an
unbounded outer retry loop: each call already owns its attempt budget.

## Testing

Use `net/http/httptest` with `WithBaseURL` to exercise HTTP behavior without
contacting NWS. Inject an `http.RoundTripper` through `WithHTTPClient` when a
test needs transport or response-stream failures.

## Contributing

Bug reports and pull requests are welcome on GitHub at [https://github.com/Three-Ships/national-weather-service-client](https://github.com/Three-Ships/national-weather-service-client).

Use the Go version in `.tool-versions` and run `make check` before submitting
changes. `make fmt` formats Go files, `make cover` writes a coverage profile, and
`lefthook install` enables the pre-commit checks. Keep tests beside their source
in matching `_test.go` files, with named tables for related cases.

## License

This package is available under the [MIT License](LICENSE.txt).
