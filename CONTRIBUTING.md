# Adding NWS endpoints

The public API lives in package `nws`. Resource services group operations by NWS
resource; they share the `Client` that `NewClient` binds to them.

See the README's [code organization](README.md#code-organization) for the file map
and the boundary between endpoint operations and shared HTTP policy.

## Add an operation to an existing service

1. Check the [NWS OpenAPI document](https://api.weather.gov/openapi.json) for the
   route, response envelope, nullable fields, and query parameters. Check existing
   types before introducing new ones.
2. Add the operation as a method on the resource service, with
   `ctx context.Context` first. Put required route inputs next and optional query
   parameters in `url.Values`, following neighboring methods.
3. Validate route inputs before making a request. Use
   `endpoint.ValidateSegment` for identifiers and `url.PathEscape` when inserting
   them into paths. Accept discovered URLs where the endpoint's contract calls
   for following NWS links; `Client.Get` validates their origin and path prefix.
4. Decode through `s.client.Get`. Do not create another client or implement
   separate HTTP, retry, or pacing logic in a service.
5. Add response types beside the operation, reusing `Feature[T]`,
   `FeatureCollection[T]`, and `QuantitativeValue` where appropriate. Use specific
   names such as `ZoneForecast` rather than a package-wide `Forecast`. Document
   the source schema, preserve nullable values, and leave source units and
   timestamps unchanged.
6. Add an external-package test in the matching `_test.go` file. Use an
   `httptest` server and call the public `client.Service.Method` API. Cover the
   route, query, decoding, and invalid inputs that must fail before HTTP. Store
   fixtures under `testdata/<resource>/`. Examples must also run without NWS.

For example, an existing operation follows this shape:

```go
func (s *StationsService) Get(ctx context.Context, id string) (*Feature[Station], error) {
    if err := endpoint.ValidateSegment(id); err != nil {
        return nil, err
    }
    var result Feature[Station]
    if err := s.client.Get(ctx, "stations/"+url.PathEscape(id), nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}
```

List methods return one page. Preserve continuation links instead of hiding an
unbounded traversal inside a request.

## Add a resource service

Create a resource-named file with an exported `<Resource>Service` struct holding
an unexported `client *Client`. Add its exported field to `Client` and initialize
it in `NewClient` with that same client, following `PointsService`. Extend the
service-policy test in `client_services_test.go` and document the new service in
`README.md` and `doc.go`. No registration framework or service constructor is
needed.

Run `make check` before submitting changes. It checks formatting, vets, builds,
and runs tests with the race detector.
