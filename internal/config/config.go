// Package config contains the private implementation of public functional options.
package config

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"
)

// LogLevel controls SDK-originated log records.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelOff   LogLevel = "off"
)

// RetryPolicy describes retry classification and delay behavior.
type RetryPolicy struct {
	// MaxRetries is the number of retries after the initial attempt.
	MaxRetries int
	// BackoffInitial is the first exponential fallback delay.
	BackoffInitial time.Duration
	// BackoffMax caps exponential fallback delays.
	BackoffMax time.Duration
	// BackoffJitter is the maximum randomly subtracted fraction.
	BackoffJitter float64
	// HTTPStatuses lists response statuses eligible for retry.
	HTTPStatuses []int
	// RespectRetryAfter enables server-directed retry delays.
	RespectRetryAfter bool
	// MaxRetryAfter caps server-directed automatic waits.
	MaxRetryAfter time.Duration
	// RetryConnectionErrors enables retries for connection and body-read failures.
	RetryConnectionErrors bool
	// RetryTimeouts enables retries for per-attempt timeouts.
	RetryTimeouts bool
}

func DefaultRetryPolicy() RetryPolicy {
	statuses := make([]int, 0, 102)
	statuses = append(statuses, http.StatusRequestTimeout, http.StatusTooManyRequests)
	for status := 500; status <= 599; status++ {
		statuses = append(statuses, status)
	}
	return RetryPolicy{
		MaxRetries: 2, BackoffInitial: 500 * time.Millisecond, BackoffMax: 5 * time.Second,
		BackoffJitter: .25, HTTPStatuses: statuses, RespectRetryAfter: true,
		MaxRetryAfter: 60 * time.Second, RetryConnectionErrors: true, RetryTimeouts: true,
	}
}

func CloneRetryPolicy(p RetryPolicy) RetryPolicy {
	p.HTTPStatuses = append([]int(nil), p.HTTPStatuses...)
	return p
}

type ClientConfig struct {
	APIKey, BaseURL, DefaultModel string
	Timeout                       time.Duration
	Retry                         RetryPolicy
	Headers                       http.Header
	HTTPClient                    *http.Client
	Logger                        *slog.Logger
	LogLevel                      LogLevel
}

type RequestConfig struct {
	Timeout      time.Duration
	Retry        RetryPolicy
	Headers      http.Header
	ResponseInto **http.Response
}

type ClientOption interface{ applyClient(*ClientConfig) error }
type RequestOption interface{ applyRequest(*RequestConfig) error }
type CommonOption interface {
	ClientOption
	RequestOption
}

type clientOptionFunc func(*ClientConfig) error

func (f clientOptionFunc) applyClient(c *ClientConfig) error { return f(c) }

type requestOptionFunc func(*RequestConfig) error

func (f requestOptionFunc) applyRequest(c *RequestConfig) error { return f(c) }

type commonOption struct {
	client  func(*ClientConfig) error
	request func(*RequestConfig) error
}

func (o commonOption) applyClient(c *ClientConfig) error   { return o.client(c) }
func (o commonOption) applyRequest(c *RequestConfig) error { return o.request(c) }

func NewClientOption(f func(*ClientConfig) error) ClientOption    { return clientOptionFunc(f) }
func NewRequestOption(f func(*RequestConfig) error) RequestOption { return requestOptionFunc(f) }
func NewCommonOption(cf func(*ClientConfig) error, rf func(*RequestConfig) error) CommonOption {
	return commonOption{client: cf, request: rf}
}

func ApplyClient(c *ClientConfig, opts []ClientOption) error {
	for i, opt := range opts {
		if opt == nil {
			return fmt.Errorf("client option %d is nil", i)
		}
		if err := opt.applyClient(c); err != nil {
			return err
		}
	}
	return nil
}

func ApplyRequest(c *RequestConfig, opts []RequestOption) error {
	for i, opt := range opts {
		if opt == nil {
			return fmt.Errorf("request option %d is nil", i)
		}
		if err := opt.applyRequest(c); err != nil {
			return err
		}
	}
	return nil
}

func ValidateRetry(p RetryPolicy) error {
	if p.MaxRetries < 0 {
		return fmt.Errorf("maximum retries must be non-negative")
	}
	if p.BackoffInitial < 0 || p.BackoffMax < 0 || p.MaxRetryAfter < 0 {
		return fmt.Errorf("retry durations must be non-negative")
	}
	if math.IsNaN(p.BackoffJitter) || math.IsInf(p.BackoffJitter, 0) || p.BackoffJitter < 0 || p.BackoffJitter > 1 {
		return fmt.Errorf("retry jitter must be finite and between 0 and 1")
	}
	for _, status := range p.HTTPStatuses {
		if status < 100 || status > 999 {
			return fmt.Errorf("retry status %d is not an HTTP status code", status)
		}
	}
	return nil
}

func CloneHeader(h http.Header) http.Header {
	if h == nil {
		return make(http.Header)
	}
	return h.Clone()
}
