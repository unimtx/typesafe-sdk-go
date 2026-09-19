package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/unimtx/typesafe-sdk-go/internal/config"
	"github.com/unimtx/typesafe-sdk-go/option"
)

type connectionError struct{ cause error }

func (e *connectionError) Error() string { return "TypeSafe connection error: " + e.cause.Error() }
func (e *connectionError) Unwrap() error { return e.cause }
func (e *connectionError) Is(target error) bool {
	return target == ErrConnection || target == ErrTypeSafe
}

type userAbortError struct{ cause error }

func (e *userAbortError) Error() string {
	return "TypeSafe request canceled by caller: " + e.cause.Error()
}
func (e *userAbortError) Unwrap() error { return e.cause }
func (e *userAbortError) Is(target error) bool {
	return target == ErrUserAbort || target == ErrTypeSafe
}

type attemptResult struct {
	response *http.Response
	body     []byte
	request  *http.Request
	err      error
}

func (c *Client) execute(ctx context.Context, method, path string, body []byte, cfg config.RequestConfig) ([]byte, *http.Response, error) {
	tag := fmt.Sprintf("#%d %s %s", c.requestCount.Add(1), method, path)
	for attempt := 0; ; attempt++ {
		result := c.attempt(ctx, method, path, body, cfg, attempt, tag)
		if result.err != nil {
			if ctx.Err() != nil {
				return nil, nil, &userAbortError{cause: ctx.Err()}
			}
			if attempt < cfg.Retry.MaxRetries && retryableTransport(result.err, cfg.Retry) {
				if err := c.backoff(ctx, attempt, cfg.Retry, nil, tag, result.err.Error()); err != nil {
					return nil, nil, err
				}
				continue
			}
			if result.response != nil {
				snapshot := snapshotResponse(result.response, result.body, result.request)
				setResponseDestination(cfg.ResponseInto, snapshot)
			}
			return nil, nil, result.err
		}

		snapshot := snapshotResponse(result.response, result.body, result.request)
		if result.response.StatusCode >= 200 && result.response.StatusCode < 300 {
			setResponseDestination(cfg.ResponseInto, snapshot)
			return result.body, snapshot, nil
		}
		apiErr := newAPIError(method, path, result.response, result.request, result.body, c.now())
		if attempt < cfg.Retry.MaxRetries && retryableStatus(result.response.StatusCode, cfg.Retry) {
			if err := c.backoff(ctx, attempt, cfg.Retry, result.response.Header, tag, apiErr.Error()); err != nil {
				return nil, nil, err
			}
			continue
		}
		setResponseDestination(cfg.ResponseInto, snapshot)
		return nil, snapshot, apiErr
	}
}

func (c *Client) attempt(parent context.Context, method, path string, body []byte, cfg config.RequestConfig, attempt int, tag string) attemptResult {
	ctx, cancel := context.WithTimeout(parent, cfg.Timeout)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return attemptResult{err: sdkError("create HTTP request: "+err.Error(), err)}
	}
	req.Header = config.CloneHeader(cfg.Headers)
	protectHeaders(req.Header, c.apiKey, body != nil, attempt)
	if body != nil {
		owned := bytes.Clone(body)
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(owned)), nil }
		req.ContentLength = int64(len(body))
	}
	c.log(parent, option.LogLevelDebug, tag+" -> request", "url", req.URL.String(), "headers", redactHeaders(req.Header), "body", string(body))
	started := c.now()
	res, err := c.httpClient.Do(req)
	if err != nil {
		classified := classifyTransportError(parent, ctx, cfg.Timeout, err)
		c.log(parent, option.LogLevelInfo, tag+" transport failure", "elapsed", c.now().Sub(started), "error", classified)
		return attemptResult{request: req, err: classified}
	}
	data, readErr := io.ReadAll(res.Body)
	closeErr := res.Body.Close()
	if readErr == nil {
		readErr = closeErr
	}
	if readErr != nil {
		classified := classifyTransportError(parent, ctx, cfg.Timeout, readErr)
		c.log(parent, option.LogLevelInfo, tag+" body failure", "elapsed", c.now().Sub(started), "error", classified)
		return attemptResult{response: res, body: data, request: req, err: classified}
	}
	c.log(parent, option.LogLevelInfo, tag+" <- response", "status", res.StatusCode, "elapsed", c.now().Sub(started), "request_id", headerValue(res.Header, "x-typesafe-request-id"))
	c.log(parent, option.LogLevelDebug, tag+" <- body", "headers", redactHeaders(res.Header), "body", string(data))
	return attemptResult{response: res, body: data, request: req}
}

func classifyTransportError(parent, attempt context.Context, timeout time.Duration, err error) error {
	if parent.Err() != nil {
		return &userAbortError{cause: parent.Err()}
	}
	if errors.Is(attempt.Err(), context.DeadlineExceeded) {
		return &TimeoutError{Duration: timeout, Cause: err}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &TimeoutError{Duration: timeout, Cause: err}
	}
	return &connectionError{cause: err}
}

func retryableTransport(err error, policy option.RetryPolicy) bool {
	if errors.Is(err, ErrTimeout) {
		return policy.RetryTimeouts
	}
	return errors.Is(err, ErrConnection) && policy.RetryConnectionErrors
}

func retryableStatus(status int, policy option.RetryPolicy) bool {
	for _, candidate := range policy.HTTPStatuses {
		if status == candidate {
			return true
		}
	}
	return false
}

func (c *Client) backoff(ctx context.Context, attempt int, policy option.RetryPolicy, headers http.Header, tag, reason string) error {
	delay := retryDelay(attempt, headers, policy, c.now(), c.random())
	c.log(ctx, option.LogLevelInfo, tag+" retrying", "delay", delay, "retry", attempt+1, "max_retries", policy.MaxRetries, "reason", reason)
	if err := c.wait(ctx, delay); err != nil {
		return &userAbortError{cause: err}
	}
	return nil
}

func retryDelay(attempt int, headers http.Header, policy option.RetryPolicy, now time.Time, random float64) time.Duration {
	if policy.RespectRetryAfter {
		if delay, ok := parseRetryAfter(headers, now); ok && delay <= policy.MaxRetryAfter {
			return delay
		}
	}
	if policy.BackoffInitial == 0 || policy.BackoffMax == 0 {
		return 0
	}
	delay := policy.BackoffInitial
	for i := 0; i < attempt && delay < policy.BackoffMax; i++ {
		if delay > time.Duration(math.MaxInt64/2) {
			delay = time.Duration(math.MaxInt64)
			break
		}
		delay *= 2
	}
	if delay > policy.BackoffMax {
		delay = policy.BackoffMax
	}
	if random < 0 {
		random = 0
	}
	if random > 1 {
		random = 1
	}
	nanos := float64(delay) * (1 - random*policy.BackoffJitter)
	return time.Duration(math.Round(nanos/float64(time.Millisecond))) * time.Millisecond
}

func parseRetryAfter(headers http.Header, now time.Time) (time.Duration, bool) {
	if headers == nil {
		return 0, false
	}
	if values, exists := headerValues(headers, "retry-after-ms"); exists && len(values) > 0 {
		raw := strings.TrimSpace(values[0])
		if raw == "" {
			raw = "0"
		}
		if value, err := strconv.ParseFloat(raw, 64); err == nil && !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 {
			return durationFromFloat(value, time.Millisecond)
		}
	}
	raw := strings.TrimSpace(headerValue(headers, "Retry-After"))
	if raw == "" {
		return 0, false
	}
	if value, err := strconv.ParseFloat(raw, 64); err == nil && !math.IsNaN(value) && !math.IsInf(value, 0) {
		if value < 0 {
			return 0, false
		}
		return durationFromFloat(value, time.Second)
	}
	date, err := http.ParseTime(raw)
	if err != nil {
		return 0, false
	}
	delay := date.Sub(now)
	if delay < 0 {
		delay = 0
	}
	return delay, true
}

func durationFromFloat(value float64, unit time.Duration) (time.Duration, bool) {
	nanos := value * float64(unit)
	if nanos > float64(math.MaxInt64) {
		return 0, false
	}
	return time.Duration(nanos), true
}

func protectHeaders(headers http.Header, apiKey string, hasBody bool, attempt int) {
	for _, name := range []string{"Authorization", "Accept", "User-Agent", "X-TypeSafe-SDK", "X-TypeSafe-Runtime", "Content-Type", "X-TypeSafe-Retry-Count"} {
		deleteHeader(headers, name)
	}
	identity := "typesafe-sdk-go/" + Version
	headers.Set("Authorization", "Bearer "+apiKey)
	headers.Set("Accept", "application/json")
	headers.Set("User-Agent", identity)
	headers.Set("X-TypeSafe-SDK", identity)
	headers.Set("X-TypeSafe-Runtime", runtimeHeader())
	if hasBody {
		headers.Set("Content-Type", "application/json")
	}
	if attempt > 0 {
		headers.Set("X-TypeSafe-Retry-Count", strconv.Itoa(attempt))
	}
}

func redactHeaders(headers http.Header) http.Header {
	copy := headers.Clone()
	for name := range copy {
		for _, sensitive := range []string{"Authorization", "Proxy-Authorization", "X-API-Key", "Cookie", "Set-Cookie"} {
			if strings.EqualFold(name, sensitive) {
				copy[name] = []string{"[REDACTED]"}
				break
			}
		}
	}
	return copy
}

func snapshotRequest(req *http.Request) *http.Request {
	if req == nil {
		return nil
	}
	copy := req.Clone(context.Background())
	copy.Header = redactHeaders(req.Header)
	if req.GetBody != nil {
		if body, err := req.GetBody(); err == nil {
			copy.Body = body
			copy.GetBody = req.GetBody
		}
	}
	return copy
}

func snapshotResponse(res *http.Response, data []byte, req *http.Request) *http.Response {
	if res == nil {
		return nil
	}
	copy := new(http.Response)
	*copy = *res
	copy.Header = redactHeaders(res.Header)
	copy.Body = io.NopCloser(bytes.NewReader(bytes.Clone(data)))
	copy.ContentLength = int64(len(data))
	copy.Request = snapshotRequest(req)
	return copy
}

func setResponseDestination(destination **http.Response, snapshot *http.Response) {
	if destination != nil {
		*destination = snapshot
	}
}

func parseErrorBody(data []byte) any {
	if len(data) == 0 {
		return nil
	}
	var body any
	if json.Unmarshal(data, &body) == nil {
		return body
	}
	return string(data)
}

func newAPIError(method, path string, res *http.Response, req *http.Request, data []byte, now time.Time) *APIError {
	requestID := headerValue(res.Header, "x-typesafe-request-id")
	body := parseErrorBody(data)
	snapshot := snapshotResponse(res, data, req)
	publicRequest := snapshotRequest(req)
	var retryAfter *time.Duration
	if res.StatusCode == http.StatusTooManyRequests {
		if delay, ok := parseRetryAfter(res.Header, now); ok {
			retryAfter = &delay
		}
	}
	err := &APIError{StatusCode: res.StatusCode, RequestID: requestID, Header: redactHeaders(res.Header), Body: body,
		RetryAfter: retryAfter, Request: publicRequest, Response: snapshot,
		message: apiErrorMessage(method, path, res.StatusCode, requestID, body, data)}
	err.requestDump = snapshotRequest(req)
	err.responseDump = snapshotResponse(res, data, req)
	return err
}

func headerValues(headers http.Header, name string) ([]string, bool) {
	for key, values := range headers {
		if strings.EqualFold(key, name) {
			return values, true
		}
	}
	return nil, false
}

func headerValue(headers http.Header, name string) string {
	values, ok := headerValues(headers, name)
	if !ok || len(values) == 0 {
		return ""
	}
	return values[0]
}

func deleteHeader(headers http.Header, name string) {
	for key := range headers {
		if strings.EqualFold(key, name) {
			delete(headers, key)
		}
	}
}
