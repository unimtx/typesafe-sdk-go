package typesafe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unimtx/typesafe-sdk-go/option"
)

func TestBodyReadFailureRetriesSameSerializedBytes(t *testing.T) {
	clearTypeSafeEnv(t)
	var calls atomic.Int32
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: &failingBody{data: []byte(`{"models":[`), err: errors.New("stream broke")}, Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"models":[]}`)), Request: r}, nil
	})
	c, _ := NewClient(option.WithAPIKey("k"), option.WithHTTPClient(&http.Client{Transport: transport}), option.WithMaxRetries(1), option.WithRetryPolicy(func(p *option.RetryPolicy) { p.BackoffInitial, p.BackoffMax = 0, 0 }))
	models, err := c.Models.List(context.Background())
	if err != nil || models == nil || calls.Load() != 2 {
		t.Fatalf("models=%v calls=%d err=%v", models, calls.Load(), err)
	}
}

func TestBodyReadFailureFinalSnapshotHasPartialBytes(t *testing.T) {
	clearTypeSafeEnv(t)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"X-Meta": {"yes"}}, Body: &failingBody{data: []byte("partial"), err: errors.New("stream broke")}, Request: r}, nil
	})
	c, _ := NewClient(option.WithAPIKey("k"), option.WithHTTPClient(&http.Client{Transport: transport}), option.WithMaxRetries(0))
	var snapshot *http.Response
	_, err := c.Models.List(context.Background(), option.WithResponseInto(&snapshot))
	if !errors.Is(err, ErrConnection) || snapshot == nil || snapshot.Header.Get("X-Meta") != "yes" {
		t.Fatalf("err=%v snapshot=%v", err, snapshot)
	}
	data, _ := io.ReadAll(snapshot.Body)
	if string(data) != "partial" {
		t.Fatalf("partial=%q", data)
	}
}

func TestSerializedBodyReusedAcrossStatusRetry(t *testing.T) {
	clearTypeSafeEnv(t)
	marshalCalls.Store(0)
	var calls int
	var bodies [][]byte
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		bodies = append(bodies, data)
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 503, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"message":"retry"}`)), Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"model":"m","answers":{"q":{"type":"noul","noul":1}},"usage":{}}`)), Request: r}, nil
	})
	c, _ := NewClient(option.WithAPIKey("k"), option.WithHTTPClient(&http.Client{Transport: transport}), option.WithMaxRetries(1), option.WithRetryPolicy(func(p *option.RetryPolicy) { p.BackoffInitial, p.BackoffMax = 0, 0 }))
	_, err := c.SystemOne(context.Background(), SystemOneRequest{State: countingMarshaler{}, Questions: Questions{"q": Noul("?")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || !bytes.Equal(bodies[0], bodies[1]) {
		t.Fatal("serialized body changed across retries")
	}
	if marshalCalls.Load() != 1 {
		t.Fatalf("caller marshaler ran %d times", marshalCalls.Load())
	}
}

func TestNilOptionsClientsAndDestinations(t *testing.T) {
	clearTypeSafeEnv(t)
	var clientOption option.ClientOption
	if _, err := NewClient(option.WithAPIKey("k"), clientOption); !errors.Is(err, ErrTypeSafe) {
		t.Fatalf("nil client option: %v", err)
	}
	c, _ := NewClient(option.WithAPIKey("k"))
	var requestOption option.RequestOption
	if _, err := c.Models.List(context.Background(), requestOption); !errors.Is(err, ErrTypeSafe) {
		t.Fatalf("nil request option: %v", err)
	}
	if _, err := c.Models.List(context.Background(), option.WithResponseInto(nil)); !errors.Is(err, ErrTypeSafe) {
		t.Fatalf("nil response destination: %v", err)
	}
	var nilClient *Client
	if _, err := nilClient.SystemOne(context.Background(), SystemOneRequest{}); !errors.Is(err, ErrTypeSafe) {
		t.Fatalf("nil client: %v", err)
	}
	if _, err := (&Client{}).SystemOne(context.Background(), SystemOneRequest{}); !errors.Is(err, ErrTypeSafe) {
		t.Fatalf("zero client: %v", err)
	}
	if _, err := (ModelService{}).List(context.Background()); !errors.Is(err, ErrTypeSafe) {
		t.Fatalf("zero service: %v", err)
	}
}

func TestResponseDestinationResetOnLocalFailure(t *testing.T) {
	clearTypeSafeEnv(t)
	c, _ := NewClient(option.WithAPIKey("k"))
	destination := &http.Response{StatusCode: 999}
	_, err := c.SystemOne(context.Background(), SystemOneRequest{State: make(chan int), Questions: Questions{"q": Noul("?")}}, option.WithResponseInto(&destination))
	if !errors.Is(err, ErrTypeSafe) || destination != nil {
		t.Fatalf("err=%v destination=%v", err, destination)
	}
}

func TestCallerDeadlineIsNotAttemptTimeout(t *testing.T) {
	clearTypeSafeEnv(t)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	c, _ := NewClient(option.WithAPIKey("k"), option.WithHTTPClient(&http.Client{Transport: transport}), option.WithTimeout(time.Hour))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := c.Models.List(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrUserAbort) || errors.Is(err, ErrTimeout) {
		t.Fatalf("caller deadline=%v", err)
	}
}

type failingBody struct {
	data   []byte
	err    error
	read   bool
	closed bool
}

func (b *failingBody) Read(p []byte) (int, error) {
	if !b.read {
		b.read = true
		n := copy(p, b.data)
		return n, nil
	}
	return 0, b.err
}
func (b *failingBody) Close() error { b.closed = true; return nil }

type countingMarshaler struct{}

var marshalCalls atomic.Int32

func (countingMarshaler) MarshalJSON() ([]byte, error) {
	marshalCalls.Add(1)
	return []byte(`{"value":"stable"}`), nil
}
