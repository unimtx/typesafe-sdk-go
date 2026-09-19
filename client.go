package typesafe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/unimtx/typesafe-sdk-go/internal/config"
	"github.com/unimtx/typesafe-sdk-go/option"
)

const (
	envAPIKey       = "TYPESAFE_API_KEY"
	envBaseURL      = "TYPESAFE_BASE_URL"
	envDefaultModel = "TYPESAFE_DEFAULT_MODEL"
	envLogLevel     = "TYPESAFE_LOG_LEVEL"

	minChoiceCriteria = 1
	maxChoiceCriteria = 255
	minScoreCriteria  = 2
	maxScoreCriteria  = 10
)

// Client is an immutable, concurrency-safe TypeSafe API client when injected
// components are safe for concurrent use. It is unofficial and unaffiliated
// with TypeSafe. Do not reassign Models while calls are active.
type Client struct {
	apiKey       string
	baseURL      string
	defaultModel string
	timeout      time.Duration
	retry        option.RetryPolicy
	headers      http.Header
	httpClient   *http.Client
	logger       *slog.Logger
	logLevel     option.LogLevel
	requestCount *atomic.Uint64
	now          func() time.Time
	random       func() float64
	wait         func(context.Context, time.Duration) error

	// Models provides model discovery. Do not reassign it while calls are active.
	Models ModelService
}

// ModelService provides model discovery methods.
type ModelService struct{ client *Client }

// NewClient reads the TypeSafe environment once, applies options in order, and
// validates configuration without network activity.
func NewClient(opts ...option.ClientOption) (*Client, error) {
	cfg := config.ClientConfig{
		Timeout:    10 * time.Second,
		Retry:      config.DefaultRetryPolicy(),
		Headers:    make(http.Header),
		HTTPClient: &http.Client{},
		Logger:     slog.New(slog.DiscardHandler),
		LogLevel:   option.LogLevelWarn,
	}
	if value, ok := readEnv(envAPIKey); ok {
		cfg.APIKey = value
	}
	if value, ok := readEnv(envBaseURL); ok {
		cfg.BaseURL = value
	} else {
		cfg.BaseURL = DefaultBaseURL
	}
	if value, ok := readEnv(envDefaultModel); ok {
		cfg.DefaultModel = value
	} else {
		cfg.DefaultModel = DefaultModel
	}
	if value, ok := readEnv(envLogLevel); ok {
		cfg.LogLevel = option.LogLevel(value)
	}
	if err := config.ApplyClient(&cfg, opts); err != nil {
		return nil, sdkError("invalid client option: "+err.Error(), err)
	}
	if err := validateClientConfig(&cfg); err != nil {
		return nil, sdkError(err.Error(), nil)
	}
	c := &Client{
		apiKey:       cfg.APIKey,
		baseURL:      strings.TrimRight(cfg.BaseURL, "/"),
		defaultModel: cfg.DefaultModel,
		timeout:      cfg.Timeout,
		retry:        config.CloneRetryPolicy(cfg.Retry),
		headers:      config.CloneHeader(cfg.Headers),
		httpClient:   cfg.HTTPClient,
		logger:       cfg.Logger,
		logLevel:     cfg.LogLevel,
		requestCount: new(atomic.Uint64),
		now:          time.Now,
		random:       defaultRandom,
		wait:         waitContext,
	}
	c.Models = ModelService{client: c}
	return c, nil
}

func readEnv(name string) (string, bool) {
	value, ok := os.LookupEnv(name)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func validateClientConfig(c *config.ClientConfig) error {
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("no API key was provided; use option.WithAPIKey or set %s", envAPIKey)
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("base URL must not be blank")
	}
	parsed, err := url.Parse(c.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("base URL must be an absolute HTTP(S) URL without userinfo, query, or fragment")
	}
	if strings.TrimSpace(c.DefaultModel) == "" {
		return fmt.Errorf("default model must not be blank")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if err := config.ValidateRetry(c.Retry); err != nil {
		return err
	}
	if c.HTTPClient == nil {
		return fmt.Errorf("HTTP client is nil")
	}
	if c.Logger == nil {
		return fmt.Errorf("logger is nil")
	}
	if !validLogLevel(c.LogLevel) {
		return fmt.Errorf("invalid log level %q; expected debug, info, warn, error, or off", c.LogLevel)
	}
	if err := validateHeaders(c.Headers); err != nil {
		return err
	}
	return nil
}

func validLogLevel(level option.LogLevel) bool {
	return level == option.LogLevelDebug || level == option.LogLevelInfo || level == option.LogLevelWarn || level == option.LogLevelError || level == option.LogLevelOff
}

// BaseURL returns the validated API root.
func (c *Client) BaseURL() string {
	if c == nil {
		return ""
	}
	return c.baseURL
}

// DefaultModel returns the model used by requests whose Model is empty.
func (c *Client) DefaultModel() string {
	if c == nil {
		return ""
	}
	return c.defaultModel
}

// Timeout returns the configured per-attempt timeout.
func (c *Client) Timeout() time.Duration {
	if c == nil {
		return 0
	}
	return c.timeout
}

// RetryPolicy returns a defensive copy of the configured policy.
func (c *Client) RetryPolicy() option.RetryPolicy {
	if c == nil {
		return option.RetryPolicy{}
	}
	return config.CloneRetryPolicy(c.retry)
}

// String returns a credential-safe client summary.
func (c Client) String() string {
	return fmt.Sprintf("typesafe.Client{BaseURL:%q, DefaultModel:%q}", c.baseURL, c.defaultModel)
}

// GoString returns a credential-safe Go-syntax client summary.
func (c Client) GoString() string { return c.String() }

// LogValue returns a credential-safe structured client value.
func (c Client) LogValue() slog.Value {
	return slog.GroupValue(slog.String("base_url", c.baseURL), slog.String("default_model", c.defaultModel))
}

// SystemOne evaluates named questions about shared state.
func (c *Client) SystemOne(ctx context.Context, req SystemOneRequest, opts ...option.RequestOption) (*SystemOneResponse, error) {
	if c == nil || c.httpClient == nil || c.requestCount == nil {
		return nil, sdkError("cannot use a nil or zero TypeSafe client", nil)
	}
	if ctx == nil {
		return nil, sdkError("context is nil", nil)
	}
	resolved, err := c.resolveRequest(opts)
	if err != nil {
		return nil, err
	}
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}
	payload := struct {
		State     Entry     `json:"state"`
		Questions Questions `json:"questions"`
		Model     string    `json:"model"`
	}{req.State, req.Questions, model}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, sdkError("encode SystemOne request: "+err.Error(), err)
	}
	if err := validateSystemOnePayload(body, req.Questions); err != nil {
		return nil, sdkError(err.Error(), err)
	}
	data, response, err := c.execute(ctx, http.MethodPost, "/v1/systemone", body, resolved)
	if err != nil {
		return nil, err
	}
	requestID := headerValue(response.Header, "x-typesafe-request-id")
	result, err := decodeSystemOne(data, requestID)
	if err != nil {
		return nil, newResponseValidationError(http.MethodPost, "/v1/systemone", response, data, err)
	}
	if err := validateAnswerKinds(result.Answers, req.Questions); err != nil {
		return nil, newResponseValidationError(http.MethodPost, "/v1/systemone", response, data, err)
	}
	return result, nil
}

// List returns the models available to the account.
func (s ModelService) List(ctx context.Context, opts ...option.RequestOption) ([]ModelCard, error) {
	if s.client == nil {
		return nil, sdkError("cannot use a zero ModelService", nil)
	}
	if ctx == nil {
		return nil, sdkError("context is nil", nil)
	}
	resolved, err := s.client.resolveRequest(opts)
	if err != nil {
		return nil, err
	}
	data, response, err := s.client.execute(ctx, http.MethodGet, "/v1/models", nil, resolved)
	if err != nil {
		return nil, err
	}
	models, err := decodeModels(data)
	if err != nil {
		return nil, newResponseValidationError(http.MethodGet, "/v1/models", response, data, err)
	}
	return models, nil
}

// String returns a credential-safe service summary.
func (s ModelService) String() string { return "typesafe.ModelService" }

// GoString returns a credential-safe Go-syntax service summary.
func (s ModelService) GoString() string { return "typesafe.ModelService{}" }

// LogValue returns a credential-safe structured service value.
func (s ModelService) LogValue() slog.Value { return slog.StringValue("typesafe.ModelService") }

func (c *Client) resolveRequest(opts []option.RequestOption) (config.RequestConfig, error) {
	resolved := config.RequestConfig{Timeout: c.timeout, Retry: config.CloneRetryPolicy(c.retry), Headers: config.CloneHeader(c.headers)}
	if err := config.ApplyRequest(&resolved, opts); err != nil {
		if resolved.ResponseInto != nil {
			*resolved.ResponseInto = nil
		}
		return resolved, sdkError("invalid request option: "+err.Error(), err)
	}
	if resolved.ResponseInto != nil {
		*resolved.ResponseInto = nil
	}
	if resolved.Timeout <= 0 {
		return resolved, sdkError("timeout must be positive", nil)
	}
	if err := config.ValidateRetry(resolved.Retry); err != nil {
		return resolved, sdkError(err.Error(), err)
	}
	if err := validateHeaders(resolved.Headers); err != nil {
		return resolved, sdkError(err.Error(), err)
	}
	resolved.Retry = config.CloneRetryPolicy(resolved.Retry)
	resolved.Headers = config.CloneHeader(resolved.Headers)
	return resolved, nil
}

func validateSystemOnePayload(body []byte, original Questions) error {
	if len(original) == 0 {
		return fmt.Errorf("at least one question is required")
	}
	ids := make([]string, 0, len(original))
	for id := range original {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if id == "" {
			return fmt.Errorf("question name must not be empty")
		}
		question := original[id]
		if question == nil {
			return fmt.Errorf("question %q is nil", id)
		}
		v := reflect.ValueOf(question)
		if v.Kind() == reflect.Pointer && v.IsNil() {
			return fmt.Errorf("question %q is a nil pointer", id)
		}
	}
	var payload struct {
		State     json.RawMessage            `json:"state"`
		Questions map[string]json.RawMessage `json:"questions"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("inspect serialized request: %w", err)
	}
	trimmed := strings.TrimSpace(string(payload.State))
	if trimmed == "" || (trimmed[0] != '"' && trimmed[0] != '{' && trimmed[0] != '[') {
		return fmt.Errorf("state must encode as a string, object, or array")
	}
	for _, id := range ids {
		raw := payload.Questions[id]
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
			return fmt.Errorf("question %q must encode as an object", id)
		}
		var kind string
		if typeRaw, ok := fields["type"]; !ok || json.Unmarshal(typeRaw, &kind) != nil || kind == "" {
			return fmt.Errorf("question %q has a missing, invalid, or empty type", id)
		}
		switch kind {
		case "choice":
			criteria, ok := fields["criteria"]
			if !ok {
				return fmt.Errorf("choice question %q has no criteria", id)
			}
			var options map[string]json.RawMessage
			if err := json.Unmarshal(criteria, &options); err != nil || options == nil {
				return fmt.Errorf("choice question %q criteria must be an object", id)
			}
			if len(options) < minChoiceCriteria || len(options) > maxChoiceCriteria {
				return fmt.Errorf("choice question %q has %d criteria; between %d and %d choices are required", id, len(options), minChoiceCriteria, maxChoiceCriteria)
			}
		case "score":
			criteria, ok := fields["criteria"]
			if !ok {
				return fmt.Errorf("score question %q has no criteria", id)
			}
			var levels []json.RawMessage
			if err := json.Unmarshal(criteria, &levels); err != nil {
				return fmt.Errorf("score question %q criteria must be an array", id)
			}
			if len(levels) < minScoreCriteria || len(levels) > maxScoreCriteria {
				return fmt.Errorf("score question %q has %d criteria; between %d and %d scores are required", id, len(levels), minScoreCriteria, maxScoreCriteria)
			}
		case "noul":
			if !meaningfulJSON(fields["instructions"]) && !meaningfulNoulCriteria(fields["criteria"]) {
				return fmt.Errorf("noul question %q has neither instructions nor criteria", id)
			}
		}
	}
	return nil
}

func meaningfulNoulCriteria(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed[0] != '{' {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return false
	}
	return meaningfulJSON(fields["true"]) || meaningfulJSON(fields["false"])
}

func meaningfulJSON(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func runtimeHeader() string {
	version := strings.TrimPrefix(runtime.Version(), "go")
	return fmt.Sprintf("go/%s (%s; %s)", version, runtime.GOOS, runtime.GOARCH)
}

func defaultRandom() float64 {
	// A per-call non-cryptographic source is enough for retry desynchronization and
	// avoids mutable package-level state. The nanosecond mix stays in [0,1).
	return float64(time.Now().UnixNano()&((1<<53)-1)) / float64(1<<53)
}

func waitContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func validateHeaders(headers http.Header) error {
	for name, values := range headers {
		if !validHeaderName(name) {
			return fmt.Errorf("invalid HTTP header name %q", name)
		}
		for _, value := range values {
			if !validHeaderValue(value) {
				return fmt.Errorf("invalid value for HTTP header %q", name)
			}
		}
	}
	return nil
}

func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		b := name[i]
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", rune(b)) {
			continue
		}
		return false
	}
	return true
}

func validHeaderValue(value string) bool {
	for i := 0; i < len(value); i++ {
		b := value[i]
		if b == '\t' || (b >= 0x20 && b != 0x7f) {
			continue
		}
		return false
	}
	return true
}

func (c *Client) logEnabled(level option.LogLevel) bool {
	rank := map[option.LogLevel]int{option.LogLevelDebug: 0, option.LogLevelInfo: 1, option.LogLevelWarn: 2, option.LogLevelError: 3, option.LogLevelOff: 4}
	return c != nil && rank[level] >= rank[c.logLevel] && c.logLevel != option.LogLevelOff
}

func (c *Client) log(ctx context.Context, level option.LogLevel, message string, attrs ...any) {
	if !c.logEnabled(level) {
		return
	}
	var slogLevel slog.Level
	switch level {
	case option.LogLevelDebug:
		slogLevel = slog.LevelDebug
	case option.LogLevelInfo:
		slogLevel = slog.LevelInfo
	case option.LogLevelWarn:
		slogLevel = slog.LevelWarn
	default:
		slogLevel = slog.LevelError
	}
	c.logger.Log(ctx, slogLevel, message, attrs...)
}

var _ io.Reader = (*memoryBody)(nil)
