// Package option provides scoped functional options for the unofficial TypeSafe
// Go SDK. This project is unaffiliated with TypeSafe.
package option

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/unimtx/typesafe-sdk-go/internal/config"
)

// ClientOption configures construction-only client state. Implementations are sealed.
type ClientOption = config.ClientOption

// RequestOption configures one network call. Implementations are sealed.
type RequestOption = config.RequestOption

// CommonOption is accepted at both client construction and individual calls.
type CommonOption = config.CommonOption

// LogLevel controls which SDK records are emitted.
type LogLevel = config.LogLevel

const (
	// LogLevelDebug emits request headers and bodies in addition to summaries.
	LogLevelDebug = config.LogLevelDebug
	// LogLevelInfo emits request and retry summaries.
	LogLevelInfo = config.LogLevelInfo
	// LogLevelWarn suppresses the SDK's info and debug records.
	LogLevelWarn = config.LogLevelWarn
	// LogLevelError suppresses all but SDK error-level records.
	LogLevelError = config.LogLevelError
	// LogLevelOff disables SDK-originated records.
	LogLevelOff = config.LogLevelOff
)

// RetryPolicy controls retries after the initial attempt. HTTPStatuses is copied
// whenever a policy enters or leaves a Client.
type RetryPolicy = config.RetryPolicy

// DefaultRetryPolicy returns a fresh policy with two retries, exponential
// backoff, and retry classification matching the pinned TypeSafe JS SDK.
func DefaultRetryPolicy() RetryPolicy { return config.DefaultRetryPolicy() }

// WithAPIKey supplies the credential used by the client.
func WithAPIKey(value string) ClientOption {
	return config.NewClientOption(func(c *config.ClientConfig) error { c.APIKey = value; return nil })
}

// WithBaseURL supplies an HTTP(S) API root. A path prefix is allowed.
func WithBaseURL(value string) ClientOption {
	return config.NewClientOption(func(c *config.ClientConfig) error { c.BaseURL = value; return nil })
}

// WithDefaultModel sets the model selected by an empty request Model.
func WithDefaultModel(value string) ClientOption {
	return config.NewClientOption(func(c *config.ClientConfig) error { c.DefaultModel = value; return nil })
}

// WithTimeout sets the timeout for a complete individual HTTP attempt.
func WithTimeout(value time.Duration) CommonOption {
	return config.NewCommonOption(
		func(c *config.ClientConfig) error { c.Timeout = value; return nil },
		func(c *config.RequestConfig) error { c.Timeout = value; return nil },
	)
}

// WithMaxRetries sets the number of retries after the initial attempt.
func WithMaxRetries(value int) CommonOption {
	return config.NewCommonOption(
		func(c *config.ClientConfig) error { c.Retry.MaxRetries = value; return nil },
		func(c *config.RequestConfig) error { c.Retry.MaxRetries = value; return nil },
	)
}

// WithRetryPolicy mutates an isolated copy of the current retry policy. The
// callback must not retain its argument for later mutation.
func WithRetryPolicy(update func(*RetryPolicy)) CommonOption {
	applyClient := func(c *config.ClientConfig) error {
		if update == nil {
			return fmt.Errorf("retry policy callback is nil")
		}
		p := config.CloneRetryPolicy(c.Retry)
		update(&p)
		c.Retry = config.CloneRetryPolicy(p)
		return nil
	}
	applyRequest := func(c *config.RequestConfig) error {
		if update == nil {
			return fmt.Errorf("retry policy callback is nil")
		}
		p := config.CloneRetryPolicy(c.Retry)
		update(&p)
		c.Retry = config.CloneRetryPolicy(p)
		return nil
	}
	return config.NewCommonOption(applyClient, applyRequest)
}

// WithHeader adds or replaces a header case-insensitively. Later options win.
func WithHeader(name, value string) CommonOption {
	set := func(h http.Header) { h.Set(name, value) }
	return config.NewCommonOption(
		func(c *config.ClientConfig) error { set(c.Headers); return nil },
		func(c *config.RequestConfig) error { set(c.Headers); return nil },
	)
}

// WithHTTPClient injects an HTTP client without mutating it.
func WithHTTPClient(value *http.Client) ClientOption {
	return config.NewClientOption(func(c *config.ClientConfig) error {
		if value == nil {
			return fmt.Errorf("HTTP client is nil")
		}
		c.HTTPClient = value
		return nil
	})
}

// WithLogger sets the structured logger used for SDK diagnostics.
func WithLogger(value *slog.Logger) ClientOption {
	return config.NewClientOption(func(c *config.ClientConfig) error {
		if value == nil {
			return fmt.Errorf("logger is nil")
		}
		c.Logger = value
		return nil
	})
}

// WithLogLevel sets the SDK log threshold.
func WithLogLevel(value LogLevel) ClientOption {
	return config.NewClientOption(func(c *config.ClientConfig) error { c.LogLevel = value; return nil })
}

// WithResponseInto captures a sanitized, buffered snapshot of the final response.
// The destination is reset to nil on entry. A captured Body is an independent
// in-memory reader that the caller closes and must not be shared across calls.
func WithResponseInto(destination **http.Response) RequestOption {
	return config.NewRequestOption(func(c *config.RequestConfig) error {
		if destination == nil {
			return fmt.Errorf("response destination is nil")
		}
		c.ResponseInto = destination
		return nil
	})
}
