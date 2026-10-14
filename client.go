package nws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// Client is safe for concurrent requests when constructed with NewClient.
// Reuse an instance to share connection pooling and pacing; do not copy it after use.
type Client struct {
	httpClient  *http.Client
	baseURL     *url.URL
	userAgent   string
	retry       RetryPolicy
	interval    time.Duration
	maxBytes    int64
	pacer       chan struct{}
	nextAttempt time.Time
}

// RetryPolicy bounds total HTTP attempts, including the initial request.
// InitialDelay and MaxDelay bound exponential full-jitter backoff. Retry-After
// is a minimum wait and may exceed MaxDelay; the caller's context still wins.
type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
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

// WithRetryPolicy replaces the default four attempts, 1s initial delay, and 30s
// maximum delay. All fields must be positive, with MaxDelay >= InitialDelay.
func WithRetryPolicy(policy RetryPolicy) Option {
	return func(c *Client) { c.retry = policy }
}

// WithRequestInterval sets the minimum spacing between HTTP attempt admissions,
// including retries and redirects. Zero disables pacing; the default is 200ms.
// It is not an in-flight limit or a quota shared between client instances.
func WithRequestInterval(interval time.Duration) Option {
	return func(c *Client) { c.interval = interval }
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
// replaces it. Set a context deadline to also bound pacing and retry waits.
func NewClient(userAgent string, options ...Option) (*Client, error) {
	base, _ := url.Parse("https://api.weather.gov")
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    base, userAgent: userAgent,
		retry:    RetryPolicy{MaxAttempts: 4, InitialDelay: time.Second, MaxDelay: 30 * time.Second},
		interval: 200 * time.Millisecond, maxBytes: 32 << 20,
		pacer: make(chan struct{}, 1),
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
	if c.retry.MaxAttempts < 1 || c.retry.InitialDelay <= 0 || c.retry.MaxDelay < c.retry.InitialDelay {
		return nil, errors.New("nws: invalid retry policy")
	}
	if c.interval < 0 || c.maxBytes <= 0 || c.maxBytes == int64(^uint64(0)>>1) {
		return nil, errors.New("nws: invalid request interval or response limit")
	}
	// Disable automatic redirects: every network attempt must pass through the
	// same origin checks, pacer, and bounded attempt budget.
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
// Redirects and retries share one paced attempt budget. Retries cover transport
// failures, incomplete successful-response reads, 429, and 5xx. Other HTTP statuses
// are terminal. Retry-After accepts delta-seconds and HTTP dates. On failure,
// Fetch returns a nil response; request failures are reported as RequestError.
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
	var last *RequestError
	for attempt := 1; attempt <= c.retry.MaxAttempts; attempt++ {
		if err := c.pace(ctx); err != nil {
			if last == nil {
				last = &RequestError{URL: u.String()}
			}
			last.Err, last.Retryable = err, false
			return nil, last
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", accept)
		resp, err := c.httpClient.Do(req)
		last = &RequestError{URL: u.String(), Attempts: attempt, Err: err, Retryable: true}
		var retryAfter time.Duration
		if err == nil {
			last.StatusCode = resp.StatusCode
			retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
			if isRedirect(resp.StatusCode) {
				// Go 1.24 reuses the connection only if the redirect body reaches EOF.
				// The 2 KiB drain bounds the extra read. A failed drain only prevents
				// reuse, so closing the body is enough; its error need not be returned.
				if resp.ContentLength == -1 || resp.ContentLength <= 2<<10 {
					_, _ = io.CopyN(io.Discard, resp.Body, 2<<10)
				}
				resp.Body.Close()
				next, redirectErr := resp.Location()
				if redirectErr == nil {
					next, redirectErr = c.resolve(next.String(), nil)
				}
				if redirectErr != nil {
					last.Err, last.Retryable = fmt.Errorf("invalid redirect: %w", redirectErr), false
					return nil, last
				}
				u = next
				last.Retryable = false
				if attempt < c.retry.MaxAttempts {
					if err := wait(ctx, retryAfter); err != nil {
						last.Err = err
						return nil, last
					}
					continue
				}
				last.Err = errors.New("redirect attempt budget exhausted")
				return nil, last
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, c.maxBytes+1))
				resp.Body.Close()
				if int64(len(body)) > c.maxBytes {
					last.Err, last.Retryable = errors.New("response exceeds configured size limit"), false
					return nil, last
				}
				if readErr == nil {
					return &Response{URL: u.String(), StatusCode: resp.StatusCode, Header: resp.Header, Body: body, attempts: attempt}, nil
				}
				last.Err = readErr
			} else {
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
				resp.Body.Close()
				last.Body, last.Err = string(body), readErr
				// Problem details are optional. Unreadable, HTML, or truncated bodies
				// must not change the retry decision made from the HTTP status.
				_ = json.Unmarshal(body, &last.Problem)
				last.Retryable = resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode < 600)
			}
		}
		if ctx.Err() != nil {
			last.Err, last.Retryable = ctx.Err(), false
			return nil, last
		}
		if !last.Retryable || attempt == c.retry.MaxAttempts {
			return nil, last
		}
		delay := max(c.backoff(attempt), retryAfter)
		if err := wait(ctx, delay); err != nil {
			last.Err, last.Retryable = err, false
			return nil, last
		}
	}
	return nil, last
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

func (c *Client) pace(ctx context.Context) error {
	// The gate prevents queued callers from each running a timer. Only a
	// successful wait advances nextAttempt, so cancellation consumes no slot.
	select {
	case c.pacer <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-c.pacer }()
	if err := wait(ctx, time.Until(c.nextAttempt)); err != nil {
		return err
	}
	c.nextAttempt = time.Now().Add(c.interval)
	return nil
}

func wait(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) backoff(retry int) time.Duration {
	delay := c.retry.InitialDelay
	for i := 1; i < retry && delay < c.retry.MaxDelay; i++ {
		if delay > c.retry.MaxDelay/2 {
			delay = c.retry.MaxDelay
			break
		}
		delay *= 2
	}
	return time.Duration(rand.Int64N(int64(delay)))
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseUint(value, 10, 64); err == nil || errors.Is(err, strconv.ErrRange) {
		if seconds > uint64((time.Duration(1<<63-1))/time.Second) {
			return time.Duration(1<<63 - 1)
		}
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		return max(0, date.Sub(now))
	}
	return 0
}

func validHeaderValue(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 32 || value[i] == 127 {
			return false
		}
	}
	return true
}

func isRedirect(status int) bool {
	return status == 301 || status == 302 || status == 303 || status == 307 || status == 308
}
