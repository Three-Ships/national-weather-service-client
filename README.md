# National Weather Service Go client

An unofficial Go client for the [National Weather Service API](https://api.weather.gov).
Requires Go 1.24 or later.

## Installation

```sh
go get github.com/Three-Ships/national-weather-service-client
```

## Usage

Create a client with a descriptive User-Agent and pass a context deadline to
bound each operation. `Client.Get` decodes JSON into a caller-owned destination;
`Client.Fetch` returns a buffered body and response headers.

```go
client, err := nws.NewClient("weather-reader/1.0 (weather@example.com)")
if err != nil {
    return err
}
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

var result struct {
    Glossary []struct{ Term, Definition string }
}
if err := client.Get(ctx, "/glossary", nil, &result); err != nil {
    return err
}
```

The client retries transport failures, incomplete successful-response reads,
429, and 5xx responses. Retries and validated redirects share a budget of four
HTTP attempts. Exponential backoff uses jitter and honors `Retry-After`;
the caller's context deadline also bounds retry waits.

Resource URLs and redirects must remain within the configured API origin and
path prefix. Query parameters use `url.Values`; malformed encoding in a resource
URL is rejected.

### Configuration

- `WithHTTPClient`: replace the default HTTP client, whose timeout is 30 seconds.
- `WithBaseURL`: replace the default `https://api.weather.gov` origin.
- `WithMaxResponseBytes`: replace the 32 MiB successful-response limit.
- `WithRetryPolicy`: replace the default four attempts, 1s initial delay, and 30s maximum backoff.
- `WithRequestInterval`: replace the default 200ms interval; zero disables pacing.

Failures return `*nws.RequestError`; inspect them with `errors.As` and use
`errors.Is` for cancellation. `Retryable` describes the cause, not whether attempts remain. Avoid adding an
unbounded outer retry loop; each call already owns its attempt budget.

### Concurrent workloads

Reuse a client for connection pooling and pacing. The default 200ms interval
admits about five HTTP attempts per second per client; retries and redirects
consume those admissions too. Pacing does not cap in-flight requests or coordinate
separate clients or processes. Bound worker concurrency in your application.

Set a context deadline to bound the whole operation, including pacing and retry
waits. Response limits apply per body, not to total application memory.

## Testing

Use `nwstest.Transport`, `nwstest.Response`, or `nwstest.FixtureClient` to
exercise HTTP behavior without contacting NWS. Test clients use three attempts
with nanosecond backoff and disable pacing.

## Contributing

Use the Go version in `.tool-versions` and run `make check` before submitting
changes. `lefthook install` enables the local checks.

## License

This package is available under the [MIT License](LICENSE.txt).
