package typesafe

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAPIErrorMessageExtractionAndFallback(t *testing.T) {
	cases := []struct {
		body any
		want string
	}{
		{map[string]any{"error": "plain"}, "plain"},
		{map[string]any{"error": map[string]any{"message": "nested"}}, "nested"},
		{map[string]any{"message": "message"}, "message"},
		{map[string]any{"detail": "detail"}, "detail"},
		{map[string]any{"detail": map[string]any{"message": "nested detail"}}, "nested detail"},
	}
	for _, tc := range cases {
		if got := extractErrorMessage(tc.body); got != tc.want {
			t.Errorf("extract %#v = %q, want %q", tc.body, got, tc.want)
		}
	}
	message := apiErrorMessage("GET", "/v1/models", 400, "", map[string]any{"code": float64(7)}, []byte("{ \n \"code\" : 7 }"))
	if message != `GET /v1/models: 400 {"code":7}` {
		t.Fatalf("fallback=%q", message)
	}
	long := strings.Repeat("x", 300)
	message = apiErrorMessage("GET", "/x", 400, "", map[string]any{"blob": long}, []byte(`{"blob":"`+long+`"}`))
	if !strings.HasSuffix(message, "…") || len([]rune(strings.TrimPrefix(message, "GET /x: 400 "))) != 201 {
		t.Fatalf("truncation=%q", message)
	}
}

func TestAPIErrorEmptyTextAndZeroRetryAfter(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/v1/models", nil)
	res := &http.Response{StatusCode: 429, Header: http.Header{"retry-after-ms": {"0"}}, Body: http.NoBody, Request: req}
	err := newAPIError(http.MethodGet, "/v1/models", res, req, nil, time.Time{})
	if err.Body != nil || err.RetryAfter == nil || *err.RetryAfter != 0 || !strings.Contains(err.Error(), "no body") {
		t.Fatalf("empty 429=%+v %v", err, err)
	}
	if !errors.Is(err, ErrRateLimit) {
		t.Fatal("429 category missing")
	}

	text := []byte("<h1>bad gateway</h1>")
	res = &http.Response{StatusCode: 502, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(text))), Request: req}
	err = newAPIError(http.MethodGet, "/v1/models", res, req, text, time.Time{})
	if err.Body != string(text) || !strings.Contains(err.Error(), string(text)) {
		t.Fatalf("text body=%#v error=%v", err.Body, err)
	}
}

func TestErrorDumpsDoNotConsumePublicReaders(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.test/v1/systemone", strings.NewReader(`{"state":"s"}`))
	req.Header.Set("Authorization", "Bearer secret")
	res := &http.Response{StatusCode: 400, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"message":"bad"}`)), Request: req}
	err := newAPIError(http.MethodPost, "/v1/systemone", res, req, []byte(`{"message":"bad"}`), time.Time{})
	for i := 0; i < 2; i++ {
		requestDump, dumpErr := err.DumpRequest(true)
		if dumpErr != nil {
			t.Fatal(dumpErr)
		}
		responseDump, dumpErr := err.DumpResponse(true)
		if dumpErr != nil {
			t.Fatal(dumpErr)
		}
		if strings.Contains(string(requestDump), "secret") || !strings.Contains(string(responseDump), `"message":"bad"`) {
			t.Fatal("unsafe or incomplete dumps")
		}
	}
	publicBody, _ := io.ReadAll(err.Response.Body)
	if string(publicBody) != `{"message":"bad"}` {
		t.Fatalf("public reader was consumed: %q", publicBody)
	}
}
