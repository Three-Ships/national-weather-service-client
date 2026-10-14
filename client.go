package nws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Client is safe for concurrent requests when constructed with NewClient.
// Reuse an instance to share connection pooling; do not copy it after use.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	userAgent  string
	maxBytes   int64
}

// Option configures a client. Repeated options replace earlier values.
type Option func(*Client)

// WithHTTPClient uses client's transport, cookie jar, and timeout. Its redirect
// policy is replaced on a copy, leaving the supplied client's settings unchanged.
// Shared dependencies must be safe for concurrent use. The caller owns transport
// cleanup.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) { c.httpClient = client }
}

// WithBaseURL sets an absolute HTTP(S) API origin, optionally with a path prefix.
// Discovered links must remain on this origin and inside that prefix.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL, _ = url.Parse(baseURL)
	}
}

// WithMaxResponseBytes bounds buffered successful responses (default 32 MiB).
// Larger responses fail without retrying. HTTP error bodies are capped at 64 KiB.
func WithMaxResponseBytes(limit int64) Option {
	return func(c *Client) { c.maxBytes = limit }
}

// NewClient validates request configuration before any requests. userAgent should
// identify the application and include contact details, for example
// "weather-reader/1.0 (weather@example.com)". Empty values and control characters
// are rejected. The default HTTP timeout is 30s per attempt; WithHTTPClient
// replaces it. Set a context deadline to bound the whole operation.
func NewClient(userAgent string, options ...Option) (*Client, error) {
	base, _ := url.Parse("https://api.weather.gov")
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    base, userAgent: userAgent,
		maxBytes: 32 << 20,
	}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("nws: nil option")
		}
		option(c)
	}
	if strings.TrimSpace(c.userAgent) == "" || !validHeaderValue(c.userAgent) {
		return nil, errors.New("nws: descriptive user-agent required, without control characters")
	}
	if c.baseURL == nil || (c.baseURL.Scheme != "http" && c.baseURL.Scheme != "https") ||
		c.baseURL.Host == "" || c.baseURL.User != nil || c.baseURL.RawQuery != "" || c.baseURL.Fragment != "" {
		return nil, errors.New("nws: base URL must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	if c.httpClient == nil {
		return nil, errors.New("nws: nil HTTP client")
	}
	if c.maxBytes <= 0 || c.maxBytes == int64(^uint64(0)>>1) {
		return nil, errors.New("nws: invalid response limit")
	}
	// Disable automatic redirects so the supplied HTTP client's redirect policy
	// cannot bypass the configured API origin or path prefix.
	hc := *c.httpClient
	hc.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	c.httpClient = &hc
	return c, nil
}

// Response contains the unmodified source body and final response headers.
// Bodies are buffered within the configured limit; there is nothing to close.
type Response struct {
	URL        string
	StatusCode int
	Header     http.Header
	Body       []byte
	attempts   int
}

// Fetch retrieves a buffered response, including non-JSON representations.
// resource may be an API-relative path or a discovered same-origin URL; both must
// stay inside any configured path prefix. query replaces matching parameters and
// retains the rest. Malformed URL query encoding fails before HTTP rather than
// dropping filters; parameter names and values are otherwise passed through to NWS.
// An empty accept selects application/geo+json.
//
// Each call makes at most one HTTP attempt. Redirects are not followed.
// On failure, Fetch returns a nil response and a RequestError.
func (c *Client) Fetch(ctx context.Context, resource string, query url.Values, accept string) (*Response, error) {
	u, err := c.resolve(resource, query)
	if err != nil {
		return nil, err
	}
	if accept == "" {
		accept = "application/geo+json"
	}
	if !validHeaderValue(accept) {
		return nil, errors.New("nws: invalid Accept header")
	}
	if err := ctx.Err(); err != nil {
		return nil, &RequestError{URL: u.String(), Err: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", accept)
	resp, err := c.httpClient.Do(req)
	failure := &RequestError{URL: u.String(), Attempts: 1, Err: err, Retryable: true}
	if err == nil {
		failure.StatusCode = resp.StatusCode
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, c.maxBytes+1))
			resp.Body.Close()
			if int64(len(body)) > c.maxBytes {
				failure.Err, failure.Retryable = errors.New("response exceeds configured size limit"), false
				return nil, failure
			}
			if readErr == nil {
				return &Response{URL: u.String(), StatusCode: resp.StatusCode, Header: resp.Header, Body: body, attempts: 1}, nil
			}
			failure.Err = readErr
		} else {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
			resp.Body.Close()
			failure.Body, failure.Err = string(body), readErr
			// Problem details must not change classification based on HTTP status.
			_ = json.Unmarshal(body, &failure.Problem)
			failure.Retryable = resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode < 600)
		}
	}
	if ctx.Err() != nil {
		failure.Err, failure.Retryable = ctx.Err(), false
	}
	return nil, failure
}

// Get fetches JSON using Fetch's request policy and decodes it into dst, which
// must be a non-nil pointer. GeoJSON and JSON are preferred over JSON-LD so
// resource packages receive feature envelopes when the endpoint offers them.
// Use json.RawMessage or a caller-defined struct for unmodeled fields. Decoding errors, including invalid
// destination types, return a non-retryable RequestError after the fetch.
func (c *Client) Get(ctx context.Context, resource string, query url.Values, dst any) error {
	resp, err := c.Fetch(ctx, resource, query, "application/geo+json, application/json, application/ld+json;q=0.9")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(resp.Body, dst); err != nil {
		return &RequestError{URL: resp.URL, Attempts: resp.attempts, StatusCode: resp.StatusCode, Err: fmt.Errorf("decode JSON: %w", err)}
	}
	return nil
}

func (c *Client) resolve(resource string, query url.Values) (*url.URL, error) {
	ref, err := url.Parse(resource)
	if err != nil || resource == "" {
		return nil, fmt.Errorf("nws: invalid resource %q", resource)
	}
	base := *c.baseURL
	base.Path = strings.TrimRight(base.Path, "/") + "/"
	if base.RawPath != "" {
		base.RawPath = strings.TrimRight(base.RawPath, "/") + "/"
	}
	// Strip leading slashes so API-relative paths keep the configured base prefix.
	if !ref.IsAbs() && ref.Host == "" {
		ref.Path = strings.TrimLeft(ref.Path, "/")
		ref.RawPath = strings.TrimLeft(ref.RawPath, "/")
	}
	u := base.ResolveReference(ref)
	if u.Scheme != c.baseURL.Scheme || !strings.EqualFold(u.Host, c.baseURL.Host) ||
		u.User != nil || u.Fragment != "" || !strings.HasPrefix(u.Path, base.Path) ||
		!strings.HasPrefix(path.Clean(u.Path)+"/", base.Path) {
		return nil, errors.New("nws: resource must remain within the configured API origin and path prefix")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("nws: invalid resource query: %w", err)
	}
	for key, values := range query {
		q[key] = append([]string(nil), values...)
	}
	u.RawQuery = q.Encode()
	return u, nil
}

func validHeaderValue(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 32 || value[i] == 127 {
			return false
		}
	}
	return true
}
