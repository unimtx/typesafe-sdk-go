package typesafe_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/unimtx/typesafe-sdk-go"
	"github.com/unimtx/typesafe-sdk-go/option"
)

func ExampleClient_SystemOne() {
	ctx := context.Background()
	client, err := typesafe.NewClient() // reads TYPESAFE_API_KEY
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.SystemOne(ctx, typesafe.SystemOneRequest{
		State: map[string]any{"document": "I was charged twice. Please fix this ASAP."},
		Questions: typesafe.Questions{
			"billing": typesafe.Noul("Is this ticket about billing?"),
			"tone":    typesafe.Choice("What is the customer's tone?", typesafe.ChoiceCriteria{"calm": nil, "frustrated": nil, "angry": nil}),
			"urgency": typesafe.Score("How urgent is this ticket?", "can wait", "this week", "today"),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	tone, err := resp.Choice("tone")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(tone.Choice, tone.Confidence)
}

func ExampleChoice() {
	route := typesafe.ChoiceQuestion{Instructions: "Which team should handle this?", Criteria: typesafe.ChoiceCriteria{
		"returns":  "Returns, exchanges, and damaged items",
		"shipping": "Delivery progress and missing parcels",
		"billing":  "Charges and payment problems",
	}}
	dynamic := make(typesafe.ChoiceCriteria)
	for _, name := range []string{"sales", "support"} {
		dynamic[name] = nil
	}
	_ = []typesafe.Question{route, typesafe.Choice("Route dynamically?", dynamic)}
}

func ExampleScore() {
	levels := []string{"low", "medium", "high"}
	typed := typesafe.Score("Severity?", levels...)
	structured := typesafe.ScoreQuestion{Instructions: "How severe is the issue?", Criteria: []typesafe.Entry{
		"Cosmetic only",
		map[string]any{"impact": "A feature is unavailable", "workaround": true},
		"Blocks all work",
	}}
	mixed := typesafe.Score[typesafe.Entry]("Rank?", nil, map[string]any{"level": "high"})
	_ = []typesafe.Question{typed, structured, mixed}
}

func ExampleNoul() {
	repeatContact := typesafe.NoulQuestion{
		Instructions: "Has the customer contacted support about this before?",
		Criteria:     &typesafe.NoulCriteria{True: "Mentions a previous request or ticket", False: "No indication of previous contact"},
	}
	oneSided := typesafe.NoulQuestion{Instructions: "Is this urgent?", Criteria: &typesafe.NoulCriteria{True: "Requires action today"}}
	_ = []typesafe.Question{typesafe.Noul("Is this about billing?"), repeatContact, oneSided}
}

func ExampleClient_options() {
	client, err := typesafe.NewClient(
		option.WithAPIKey("test-key"),
		option.WithDefaultModel("jev-latest"),
		option.WithTimeout(10*time.Second),
		option.WithMaxRetries(2),
		option.WithHeader("X-Trace-ID", "trace-1"),
	)
	if err != nil {
		log.Fatal(err)
	}
	_, _ = client.SystemOne(context.Background(), typesafe.SystemOneRequest{
		State:     "A request-specific model and settings",
		Questions: typesafe.Questions{"q": typesafe.Noul("Is this clear?")},
		Model:     "jev-latest",
	}, option.WithTimeout(3*time.Second), option.WithMaxRetries(0), option.WithHeader("X-Trace-ID", "trace-2"))
}

func ExampleClient_rawResponse() {
	client, err := typesafe.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	var raw *http.Response
	_, err = client.Models.List(context.Background(), option.WithResponseInto(&raw))
	if raw != nil {
		defer raw.Body.Close()
		body, _ := io.ReadAll(raw.Body) // independent in-memory snapshot
		_ = body
	}
	if err != nil {
		log.Print(err)
	}
}

func ExampleRawQuestion() {
	future := typesafe.RawQuestion{JSON: json.RawMessage(`{"type":"future_kind","instructions":"Evaluate this","future_field":true}`)}
	client, err := typesafe.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	response, err := client.SystemOne(context.Background(), typesafe.SystemOneRequest{State: "state", Questions: typesafe.Questions{"future": future}})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response.RawJSON(), response.Answers["future"].RawJSON())
}

func ExampleModelService_List() {
	client, err := typesafe.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	models, err := client.Models.List(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, model := range models {
		fmt.Println(model.Name, model.ReleaseDate)
	}
}

func ExampleAPIError() {
	client, err := typesafe.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	_, err = client.Models.List(context.Background())
	if errors.Is(err, typesafe.ErrRateLimit) {
		log.Print("retry later")
	}
	var responseErr *typesafe.ResponseValidationError
	if errors.As(err, &responseErr) {
		fmt.Println(responseErr.FieldPath, responseErr.RequestID)
	}
	var apiErr *typesafe.APIError
	if errors.As(err, &apiErr) {
		fmt.Println(apiErr.StatusCode, apiErr.RequestID, apiErr.RetryAfter)
	}
	var timeoutErr *typesafe.TimeoutError
	if errors.As(err, &timeoutErr) {
		fmt.Println(timeoutErr.Duration)
	}
}
