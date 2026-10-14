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

The client makes one HTTP attempt per operation and does not follow redirects.
Resource URLs must remain within the configured API origin and path prefix.
Query parameters use `url.Values`; malformed encoding in a resource URL is rejected.

### Configuration

- `WithHTTPClient`: replace the default HTTP client, whose timeout is 30 seconds.
- `WithBaseURL`: replace the default `https://api.weather.gov` origin.
- `WithMaxResponseBytes`: replace the 32 MiB successful-response limit.

Failures return `*nws.RequestError`; inspect them with `errors.As` and use
`errors.Is` for cancellation. `Retryable` classifies the cause but does not
initiate another attempt.

## Testing

Use `nwstest.Transport`, `nwstest.Response`, or `nwstest.FixtureClient` to
exercise HTTP behavior without contacting NWS.

## Contributing

Use the Go version in `.tool-versions` and run `make check` before submitting
changes. `lefthook install` enables the local checks.

## License

This package is available under the [MIT License](LICENSE.txt).
