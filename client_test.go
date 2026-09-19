package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unimtx/typesafe-sdk-go/option"
)

func clearTypeSafeEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{envAPIKey, envBaseURL, envDefaultModel, envLogLevel} {
		t.Setenv(name, "")
	}
}

func TestNewClientEnvironmentPrecedenceAndValidation(t *testing.T) {
	clearTypeSafeEnv(t)
	if _, err := NewClient(); !errors.Is(err, ErrTypeSafe) {
		t.Fatalf("missing key: %v", err)
	}
	t.Setenv(envAPIKey, " env-key ")
	t.Setenv(envBaseURL, " https://env.test/prefix/// ")
	t.Setenv(envDefaultModel, " env-model ")
	t.Setenv(envLogLevel, "info")
	c, err := NewClient(option.WithAPIKey("explicit"), option.WithBaseURL("https://explicit.test/v1/"), option.WithDefaultModel("explicit-model"))
	if err != nil {
		t.Fatal(err)
	}
	if c.BaseURL() != "https://explicit.test/v1" || c.DefaultModel() != "explicit-model" || c.apiKey != "explicit" {
		t.Fatalf("precedence: %v", c)
	}
	for _, opt := range []option.ClientOption{option.WithAPIKey(" "), option.WithBaseURL("relative"), option.WithDefaultModel("")} {
		if _, err := NewClient(opt); err == nil {
			t.Fatalf("accepted bad option %T", opt)
		}
	}
	if _, err := NewClient(option.WithAPIKey("k"), option.WithTimeout(0)); err == nil {
		t.Fatal("accepted zero timeout")
	}
	if _, err := NewClient(option.WithAPIKey("k"), option.WithLogLevel("INFO")); err == nil {
		t.Fatal("accepted invalid log level")
	}
	t.Setenv(envBaseURL, "not-a-url")
	t.Setenv(envLogLevel, "loud")
	if _, err := NewClient(option.WithAPIKey("k"), option.WithBaseURL("https://valid.test"), option.WithLogLevel(option.LogLevelOff)); err != nil {
		t.Fatalf("valid explicit options did not override invalid environment: %v", err)
	}
	badPolicies := []func(*option.RetryPolicy){
		func(p *option.RetryPolicy) { p.MaxRetries = -1 },
		func(p *option.RetryPolicy) { p.BackoffInitial = -1 },
		func(p *option.RetryPolicy) { p.BackoffJitter = 2 },
		func(p *option.RetryPolicy) { p.HTTPStatuses = []int{99} },
	}
	for i, update := range badPolicies {
		if _, err := NewClient(option.WithAPIKey("k"), option.WithBaseURL("https://valid.test"), option.WithLogLevel(option.LogLevelOff), option.WithRetryPolicy(update)); err == nil {
			t.Errorf("accepted invalid retry policy %d", i)
		}
	}
	if _, err := NewClient(option.WithAPIKey("k"), option.WithRetryPolicy(nil)); err == nil {
		t.Fatal("accepted nil retry callback")
	}
}

func TestSystemOneWireHeadersDecodeAndSnapshot(t *testing.T) {
	clearTypeSafeEnv(t)
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if r.URL.Path != "/prefix/v1/systemone" || r.Method != http.MethodPost {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("authorization=%q", got)
		}
		if got := r.Header.Get("X-Team"); got != "call" {
			t.Errorf("team=%q", got)
		}
		if r.Header.Get("User-Agent") != "typesafe-sdk-go/"+Version || !strings.HasPrefix(r.Header.Get("X-TypeSafe-Runtime"), "go/") {
			t.Error("identity headers missing")
		}
		if r.Header.Get("Accept") != "application/json" || r.Header.Get("X-TypeSafe-SDK") != "typesafe-sdk-go/"+Version || r.Header.Get("Content-Type") != "application/json" {
			t.Error("protected content/identity headers missing")
		}
		wantRetry := ""
		if attempt == 2 {
			wantRetry = "1"
		}
		if got := r.Header.Get("X-TypeSafe-Retry-Count"); got != wantRetry {
			t.Errorf("retry=%q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		if string(payload["model"]) != `"client-model"` {
			t.Errorf("body=%s", body)
		}
		if attempt == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":"again"}`)
			return
		}
		w.Header().Set("X-TypeSafe-Request-ID", "req_1")
		w.Header().Set("Set-Cookie", "secret-cookie")
		_, _ = io.WriteString(w, `{"model":"m","answers":{"q":{"type":"score","score":0.5,"confidence":0.7,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.5,"1":0.5}}},"usage":{"input_tokens":1,"output_tokens":2}}`)
	}))
	defer server.Close()
	client, err := NewClient(option.WithAPIKey("secret"), option.WithBaseURL(server.URL+"/prefix"), option.WithDefaultModel("client-model"), option.WithMaxRetries(1), option.WithRetryPolicy(func(p *option.RetryPolicy) { p.BackoffInitial = 0; p.BackoffMax = 0 }), option.WithHeader("X-Team", "default"), option.WithHeader("authorization", "bad"))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot *http.Response
	result, err := client.SystemOne(context.Background(), SystemOneRequest{State: map[string]any{"x": 1}, Questions: Questions{"q": Score("rank", "low", "high")}}, option.WithHeader("x-team", "call"), option.WithHeader("AUTHORIZATION", "also-bad"), option.WithResponseInto(&snapshot))
	if err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 || result.RequestID != "req_1" || result.Scores()["q"].Score != .5 {
		t.Fatalf("result=%+v attempts=%d", result, attempts.Load())
	}
	if snapshot == nil || snapshot.Request.Header.Get("Authorization") != "[REDACTED]" || snapshot.Header.Get("Set-Cookie") != "[REDACTED]" {
		t.Fatal("snapshot was not redacted")
	}
	one, _ := io.ReadAll(snapshot.Body)
	two, _ := io.ReadAll(snapshot.Body)
	if len(one) == 0 || len(two) != 0 {
		t.Fatal("snapshot body ownership mismatch")
	}
}

func TestModelsListGETAndEnvelopeValidation(t *testing.T) {
	clearTypeSafeEnv(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "" || r.Header.Get("X-TypeSafe-Retry-Count") != "" {
			t.Error("protected GET headers leaked")
		}
		_, _ = io.WriteString(w, `{"models":[{"name":"m","description":"d","release_date":"2026","future":true}]}`)
	}))
	defer server.Close()
	c, _ := NewClient(option.WithAPIKey("k"), option.WithBaseURL(server.URL), option.WithHeader("content-type", "bad"), option.WithHeader("x-typesafe-retry-count", "99"))
	var modelSnapshot *http.Response
	models, err := c.Models.List(context.Background(), option.WithResponseInto(&modelSnapshot))
	if err != nil || len(models) != 1 || models[0].Name != "m" {
		t.Fatalf("models=%v err=%v", models, err)
	}
	rawModels, _ := io.ReadAll(modelSnapshot.Body)
	if !bytes.Contains(rawModels, []byte(`"future":true`)) {
		t.Fatalf("raw model field missing: %s", rawModels)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{"models":null}`) }))
	defer bad.Close()
	c, _ = NewClient(option.WithAPIKey("k"), option.WithBaseURL(bad.URL))
	if _, err := c.Models.List(context.Background()); !errors.Is(err, ErrTypeSafe) {
		t.Fatalf("bad envelope: %v", err)
	}
}

func TestLocalValidationBeforeNetwork(t *testing.T) {
	clearTypeSafeEnv(t)
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("network called"); return nil, nil })
	c, _ := NewClient(option.WithAPIKey("k"), option.WithHTTPClient(&http.Client{Transport: transport}))
	tests := []SystemOneRequest{
		{State: nil, Questions: Questions{"q": Noul("?")}},
		{State: true, Questions: Questions{"q": Noul("?")}},
		{State: "s", Questions: nil},
		{State: "s", Questions: Questions{"q": Score[Entry]("?")}},
		{State: "s", Questions: Questions{"q": Score("?", "only")}},
		{State: "s", Questions: Questions{"q": RawQuestion{JSON: []byte(`{"type":"score","criteria":{}}`)}}},
		{State: "s", Questions: Questions{"q": RawQuestion{JSON: []byte(`{"x":1}`)}}},
	}
	for i, req := range tests {
		if _, err := c.SystemOne(context.Background(), req); !errors.Is(err, ErrTypeSafe) {
			t.Fatalf("case %d: %v", i, err)
		}
	}
	var nilQuestion *NoulQuestion
	if _, err := c.SystemOne(context.Background(), SystemOneRequest{State: "s", Questions: Questions{"q": nilQuestion}}); err == nil {
		t.Fatal("accepted typed-nil question")
	}
}

func TestModelResolutionDoesNotMutateRequest(t *testing.T) {
	clearTypeSafeEnv(t)
	var models []string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		var wire struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(body, &wire); err != nil {
			t.Fatal(err)
		}
		models = append(models, wire.Model)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"model":"m","answers":{"q":{"type":"noul","noul":1}},"usage":{}}`)), Request: r}, nil
	})
	c, _ := NewClient(option.WithAPIKey("k"), option.WithDefaultModel("client-default"), option.WithHTTPClient(&http.Client{Transport: transport}))
	req := SystemOneRequest{State: "s", Questions: Questions{"q": Noul("?")}}
	if _, err := c.SystemOne(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if req.Model != "" {
		t.Fatal("request model was mutated")
	}
	req.Model = "per-request"
	if _, err := c.SystemOne(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "client-default" || models[1] != "per-request" {
		t.Fatalf("models=%v", models)
	}
}

func TestAPIErrorCategoriesBodyAndDumps(t *testing.T) {
	clearTypeSafeEnv(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After-Ms", "61000")
		w.Header().Set("X-TypeSafe-Request-ID", "req_bad")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"detail":[{"loc":["body","questions","q"],"msg":"invalid"}]}`)
	}))
	defer server.Close()
	c, _ := NewClient(option.WithAPIKey("top-secret"), option.WithBaseURL(server.URL), option.WithMaxRetries(0))
	var snapshot *http.Response
	_, err := c.Models.List(context.Background(), option.WithResponseInto(&snapshot))
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !errors.Is(err, ErrRateLimit) || !errors.Is(err, ErrTypeSafe) {
		t.Fatalf("error=%v", err)
	}
	if apiErr.RetryAfter == nil || *apiErr.RetryAfter != 61*time.Second || apiErr.RequestID != "req_bad" || !strings.Contains(err.Error(), "questions.q: invalid") {
		t.Fatalf("api error=%+v %v", apiErr, err)
	}
	requestDump, _ := apiErr.DumpRequest(true)
	responseDump, _ := apiErr.DumpResponse(true)
	if bytes.Contains(requestDump, []byte("top-secret")) || !bytes.Contains(responseDump, []byte("invalid")) {
		t.Fatal("dump redaction/body mismatch")
	}
	if snapshot == nil {
		t.Fatal("missing response snapshot")
	}
	for status, sentinel := range map[int]error{400: ErrBadRequest, 401: ErrAuthentication, 403: ErrPermissionDenied, 404: ErrNotFound, 422: ErrUnprocessableEntity, 500: ErrInternalServer, 529: ErrInternalServer} {
		e := &APIError{StatusCode: status}
		if !errors.Is(e, sentinel) {
			t.Errorf("status %d did not match %v", status, sentinel)
		}
	}
}

func TestCancellationTimeoutAndConnectionClassification(t *testing.T) {
	clearTypeSafeEnv(t)
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
	}))
	defer slow.Close()
	c, _ := NewClient(option.WithAPIKey("k"), option.WithBaseURL(slow.URL), option.WithTimeout(20*time.Millisecond), option.WithMaxRetries(0))
	_, err := c.Models.List(context.Background())
	var timeout *TimeoutError
	if !errors.As(err, &timeout) || !errors.Is(err, ErrTimeout) || !errors.Is(err, ErrConnection) {
		t.Fatalf("timeout=%v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.Models.List(ctx)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrUserAbort) || errors.Is(err, ErrTimeout) {
		t.Fatalf("cancel=%v", err)
	}

	broken, _ := NewClient(option.WithAPIKey("k"), option.WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("boom") })}), option.WithMaxRetries(0))
	_, err = broken.Models.List(context.Background())
	if !errors.Is(err, ErrConnection) || errors.Is(err, ErrTimeout) {
		t.Fatalf("connection=%v", err)
	}
}

func TestCancelDuringBackoff(t *testing.T) {
	clearTypeSafeEnv(t)
	c, _ := NewClient(option.WithAPIKey("k"), option.WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("boom") })}), option.WithMaxRetries(1), option.WithRetryPolicy(func(p *option.RetryPolicy) { p.BackoffInitial = time.Hour }))
	ctx, cancel := context.WithCancel(context.Background())
	c.wait = func(context.Context, time.Duration) error { cancel(); return ctx.Err() }
	_, err := c.Models.List(ctx)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrUserAbort) {
		t.Fatalf("backoff cancel=%v", err)
	}
}

func TestConfigCopyIsolationAndSafeFormatting(t *testing.T) {
	clearTypeSafeEnv(t)
	statuses := []int{409}
	var retained *option.RetryPolicy
	c, err := NewClient(option.WithAPIKey("secret-value"), option.WithRetryPolicy(func(p *option.RetryPolicy) { p.HTTPStatuses = statuses; retained = p }))
	if err != nil {
		t.Fatal(err)
	}
	statuses[0] = 500
	retained.MaxRetries = 99
	got := c.RetryPolicy()
	if got.HTTPStatuses[0] != 409 || got.MaxRetries == 99 {
		t.Fatal("client retained caller status slice")
	}
	got.HTTPStatuses[0] = 401
	if c.RetryPolicy().HTTPStatuses[0] != 409 {
		t.Fatal("accessor exposed status slice")
	}
	for _, text := range []string{fmt.Sprint(*c), fmt.Sprintf("%#v", *c), fmt.Sprint(c), fmt.Sprint(c.Models)} {
		if strings.Contains(text, "secret-value") {
			t.Fatalf("credential in %q", text)
		}
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret-value") {
		t.Fatalf("credential in client JSON: %s", encoded)
	}
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger.Info("client", "value", c)
	if strings.Contains(output.String(), "secret-value") {
		t.Fatal("credential in slog")
	}
}

func TestDecodeErrorStillReturnsRawSnapshot(t *testing.T) {
	clearTypeSafeEnv(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); _, _ = io.WriteString(w, `not-json`) }))
	defer server.Close()
	c, _ := NewClient(option.WithAPIKey("k"), option.WithBaseURL(server.URL))
	var snapshot *http.Response
	_, err := c.Models.List(context.Background(), option.WithResponseInto(&snapshot))
	if !errors.Is(err, ErrTypeSafe) || snapshot == nil {
		t.Fatalf("err=%v snapshot=%v", err, snapshot)
	}
	body, _ := io.ReadAll(snapshot.Body)
	if string(body) != "not-json" {
		t.Fatalf("snapshot body=%q", body)
	}
	if calls.Load() != 1 {
		t.Fatalf("decode failure retried %d times", calls.Load())
	}
}

func TestLoggingLevelAndCredentialRedaction(t *testing.T) {
	clearTypeSafeEnv(t)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Set-Cookie": {"session-secret"}}, Body: io.NopCloser(strings.NewReader(`{"models":[]}`)), Request: r}, nil
	})
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	c, err := NewClient(option.WithAPIKey("api-secret"), option.WithHTTPClient(&http.Client{Transport: transport}), option.WithLogger(logger), option.WithLogLevel(option.LogLevelDebug))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Models.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Contains(text, "api-secret") || strings.Contains(text, "session-secret") || !strings.Contains(text, "[REDACTED]") || !strings.Contains(text, `models`) {
		t.Fatalf("unsafe or incomplete debug log: %s", text)
	}
	output.Reset()
	c, _ = NewClient(option.WithAPIKey("api-secret"), option.WithHTTPClient(&http.Client{Transport: transport}), option.WithLogger(logger), option.WithLogLevel(option.LogLevelWarn))
	if _, err := c.Models.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("warn level emitted info/debug records: %s", output.String())
	}
}

func TestConcurrentCalls(t *testing.T) {
	clearTypeSafeEnv(t)
	var calls atomic.Int32
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"models":[]}`)), Request: r}, nil
	})
	c, _ := NewClient(option.WithAPIKey("k"), option.WithHTTPClient(&http.Client{Transport: transport}))
	const count = 32
	var wg sync.WaitGroup
	errorsSeen := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := c.Models.List(context.Background()); errorsSeen <- err }()
	}
	wg.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Error(err)
		}
	}
	if calls.Load() != count {
		t.Fatalf("calls=%d want %d", calls.Load(), count)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
