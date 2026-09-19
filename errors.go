package typesafe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

var (
	// ErrTypeSafe matches every error created by this SDK.
	ErrTypeSafe = errors.New("typesafe SDK error")
	// ErrBadRequest matches HTTP 400 API errors.
	ErrBadRequest = errors.New("typesafe bad request")
	// ErrAuthentication matches HTTP 401 API errors.
	ErrAuthentication = errors.New("typesafe authentication failed")
	// ErrPermissionDenied matches HTTP 403 API errors.
	ErrPermissionDenied = errors.New("typesafe permission denied")
	// ErrNotFound matches HTTP 404 API errors.
	ErrNotFound = errors.New("typesafe resource not found")
	// ErrUnprocessableEntity matches HTTP 422 API errors.
	ErrUnprocessableEntity = errors.New("typesafe unprocessable entity")
	// ErrRateLimit matches HTTP 429 API errors.
	ErrRateLimit = errors.New("typesafe rate limit")
	// ErrInternalServer matches HTTP 5xx API errors.
	ErrInternalServer = errors.New("typesafe internal server error")
	// ErrConnection matches connection failures and SDK attempt timeouts.
	ErrConnection = errors.New("typesafe connection error")
	// ErrTimeout matches SDK per-attempt timeouts, not caller deadlines.
	ErrTimeout = errors.New("typesafe request timeout")
	// ErrUserAbort matches caller cancellation and caller deadline expiration.
	ErrUserAbort = errors.New("typesafe request canceled by caller")
	// ErrResponseValidation matches successful HTTP responses that do not satisfy
	// the documented response shape.
	ErrResponseValidation = errors.New("typesafe response validation failed")
)

// Error reports configuration, request validation, or encoding failures.
type Error struct {
	// Message is the safe, human-readable failure description.
	Message string
	// Cause is the optional underlying encoding, validation, or option error.
	Cause error
}

// Error returns the failure description.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Message
}

// Unwrap returns the underlying cause when present.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is reports membership in the ErrTypeSafe category.
func (e *Error) Is(target error) bool { return target == ErrTypeSafe }

func sdkError(message string, cause error) *Error { return &Error{Message: message, Cause: cause} }

// ResponseValidationError reports a 2xx response that could not be decoded as
// the documented result. FieldPath is "$" for the response root or an RFC 6901
// JSON Pointer for a specific field. Response is a sanitized buffered snapshot.
type ResponseValidationError struct {
	// Method is the HTTP method used for the request.
	Method string
	// Path is the API endpoint path.
	Path string
	// RequestID is the x-typesafe-request-id header value, when present.
	RequestID string
	// FieldPath identifies the invalid response location.
	FieldPath string
	// Response is a sanitized response snapshot with an independent buffered body.
	Response *http.Response
	// Cause describes the incompatible JSON value or decoding failure.
	Cause error
}

// Error returns a method/path/request-ID/field description without including
// the response body.
func (e *ResponseValidationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	prefix := strings.TrimSpace(e.Method + " " + e.Path)
	if prefix == "" {
		prefix = "TypeSafe"
	}
	message := prefix + " response validation failed"
	if e.RequestID != "" {
		message += " (request " + e.RequestID + ")"
	}
	if e.FieldPath != "" {
		message += " at " + e.FieldPath
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

// Unwrap returns the underlying decoding or shape error.
func (e *ResponseValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is reports response-validation and SDK error category membership.
func (e *ResponseValidationError) Is(target error) bool {
	return target == ErrResponseValidation || target == ErrTypeSafe
}

func newResponseValidationError(method, path string, response *http.Response, data []byte, err error) *ResponseValidationError {
	fieldPath := "$"
	cause := err
	var fieldErr *responseFieldError
	if errors.As(err, &fieldErr) {
		fieldPath = fieldErr.path
		cause = fieldErr.cause
	}
	requestID := ""
	var snapshot *http.Response
	if response != nil {
		requestID = headerValue(response.Header, "x-typesafe-request-id")
		snapshot = snapshotResponse(response, data, response.Request)
	}
	return &ResponseValidationError{
		Method: method, Path: path, RequestID: requestID, FieldPath: fieldPath,
		Response: snapshot, Cause: cause,
	}
}

// TimeoutError reports an SDK per-attempt timeout.
type TimeoutError struct {
	// Duration is the configured timeout for the failed attempt.
	Duration time.Duration
	// Cause is the underlying transport or context failure.
	Cause error
}

// Error returns the timeout description.
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("TypeSafe request timed out after %s", e.Duration)
}

// Unwrap returns the underlying timeout cause.
func (e *TimeoutError) Unwrap() error { return e.Cause }

// Is reports timeout, connection, and SDK error category membership.
func (e *TimeoutError) Is(target error) bool {
	return target == ErrTimeout || target == ErrConnection || target == ErrTypeSafe
}

// APIError reports a non-2xx HTTP response and retains sanitized HTTP snapshots.
type APIError struct {
	// StatusCode is the non-2xx HTTP status.
	StatusCode int
	// RequestID is the x-typesafe-request-id header value, when present.
	RequestID string
	// Header is a sanitized defensive copy of response headers.
	Header http.Header
	// Body is parsed JSON, response text, or nil for an empty body.
	Body any
	// RetryAfter is the valid server delay on a 429, including zero.
	RetryAfter *time.Duration
	// Request is a sanitized, detached request snapshot.
	Request *http.Request
	// Response is a sanitized response snapshot with an independent buffered body.
	Response     *http.Response
	message      string
	requestDump  *http.Request
	responseDump *http.Response
}

// Error returns a method/path/status/request-ID description and upstream detail.
func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.message
}

// Is reports SDK and status-specific error category membership.
func (e *APIError) Is(target error) bool {
	if target == ErrTypeSafe {
		return true
	}
	switch {
	case e.StatusCode == 400:
		return target == ErrBadRequest
	case e.StatusCode == 401:
		return target == ErrAuthentication
	case e.StatusCode == 403:
		return target == ErrPermissionDenied
	case e.StatusCode == 404:
		return target == ErrNotFound
	case e.StatusCode == 422:
		return target == ErrUnprocessableEntity
	case e.StatusCode == 429:
		return target == ErrRateLimit
	case e.StatusCode >= 500:
		return target == ErrInternalServer
	default:
		return false
	}
}

// DumpRequest returns a redacted dump from a private buffered snapshot.
func (e *APIError) DumpRequest(body bool) ([]byte, error) {
	if e == nil || e.requestDump == nil {
		return nil, errors.New("no request snapshot")
	}
	return httputil.DumpRequestOut(cloneRequest(e.requestDump), body)
}

// DumpResponse returns a redacted dump from a private buffered snapshot.
func (e *APIError) DumpResponse(body bool) ([]byte, error) {
	if e == nil || e.responseDump == nil {
		return nil, errors.New("no response snapshot")
	}
	return httputil.DumpResponse(cloneResponse(e.responseDump), body)
}

func cloneRequest(r *http.Request) *http.Request {
	if r == nil {
		return nil
	}
	c := r.Clone(r.Context())
	c.Header = r.Header.Clone()
	if r.GetBody != nil {
		body, err := r.GetBody()
		if err == nil {
			c.Body = body
		}
	}
	return c
}

func cloneResponse(r *http.Response) *http.Response {
	if r == nil {
		return nil
	}
	c := new(http.Response)
	*c = *r
	c.Header = r.Header.Clone()
	if r.Body != nil {
		data, _ := readAndRestore(r)
		c.Body = http.NoBody
		if len(data) > 0 {
			c.Body = ioNopCloser(data)
		}
	}
	if r.Request != nil {
		c.Request = cloneRequest(r.Request)
	}
	return c
}

func readAndRestore(r *http.Response) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	data, err := ioReadAll(r.Body)
	r.Body = ioNopCloser(data)
	return data, err
}

// Small indirections keep the snapshot helpers concentrated here.
func ioReadAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var b bytes.Buffer
	_, err := b.ReadFrom(r)
	return b.Bytes(), err
}
func ioNopCloser(data []byte) *memoryBody { return &memoryBody{Reader: *bytes.NewReader(data)} }

type memoryBody struct{ bytes.Reader }

func (*memoryBody) Close() error { return nil }

func apiErrorMessage(method, path string, status int, requestID string, body any, raw []byte) string {
	detail := extractErrorMessage(body)
	if detail == "" {
		if len(raw) == 0 {
			detail = "status code (no body)"
		} else {
			if text, ok := body.(string); ok {
				detail = text
			} else if encoded, err := json.Marshal(body); err == nil {
				detail = string(encoded)
			} else {
				detail = string(raw)
			}
			if len(detail) > 200 {
				detail = detail[:200] + "…"
			}
		}
	}
	prefix := fmt.Sprintf("%s %s: %d", method, path, status)
	if requestID != "" {
		prefix += " (request " + requestID + ")"
	}
	return prefix + " " + detail
}

func extractErrorMessage(body any) string {
	if text, ok := body.(string); ok {
		return text
	}
	m, ok := body.(map[string]any)
	if !ok {
		return ""
	}
	if s, ok := m["error"].(string); ok {
		return s
	}
	if nested, ok := m["error"].(map[string]any); ok {
		if s, ok := nested["message"].(string); ok {
			return s
		}
	}
	if s, ok := m["message"].(string); ok {
		return s
	}
	detail := m["detail"]
	if s, ok := detail.(string); ok {
		return s
	}
	if nested, ok := detail.(map[string]any); ok {
		if s, ok := nested["message"].(string); ok {
			return s
		}
	}
	items, ok := detail.([]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		v, ok := item.(map[string]any)
		if !ok {
			continue
		}
		msg, ok := v["msg"].(string)
		if !ok {
			continue
		}
		path := ""
		if loc, ok := v["loc"].([]any); ok {
			var names []string
			for _, part := range loc {
				if s := fmt.Sprint(part); s != "body" {
					names = append(names, s)
				}
			}
			path = strings.Join(names, ".")
		}
		if path != "" {
			msg = path + ": " + msg
		}
		parts = append(parts, msg)
	}
	return strings.Join(parts, "; ")
}
