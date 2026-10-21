package nws

import "fmt"

// ProblemDetail models the NWS OpenAPI ProblemDetail schema, an RFC 7807 error
// document. Its Status may differ from the actual HTTP status; use
// RequestError.StatusCode for retry decisions.
//
// Schema: https://api.weather.gov/openapi.json#/components/schemas/ProblemDetail
type ProblemDetail struct {
	Type          string `json:"type"`
	Title         string `json:"title"`
	Status        int    `json:"status"`
	Detail        string `json:"detail"`
	Instance      string `json:"instance"`
	CorrelationID string `json:"correlationId"`
}

// RequestError reports a failure after the client has applied its retry policy.
// Inspect it with errors.As; errors.Is can inspect the wrapped cause. Retryable
// describes the cause, not whether attempts remain. Attempts counts http.Client.Do
// calls, not redirect hops. StatusCode is zero when no HTTP status is available.
// Body contains at most 64 KiB of an HTTP error response.
type RequestError struct {
	URL        string
	Attempts   int
	StatusCode int
	Retryable  bool
	Problem    ProblemDetail
	Body       string
	Err        error
}

func (e *RequestError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("nws GET %s after %d attempts: %v", e.URL, e.Attempts, e.Err)
	}
	return fmt.Sprintf("nws GET %s after %d attempts: HTTP %d (%s)", e.URL, e.Attempts, e.StatusCode, e.Problem.Type)
}

// Unwrap preserves transport, decoding, and context error identity.
func (e *RequestError) Unwrap() error { return e.Err }
