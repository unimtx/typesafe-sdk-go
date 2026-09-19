//go:build integration

package integration_test

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	typesafe "github.com/unimtx/typesafe-sdk-go"
	"github.com/unimtx/typesafe-sdk-go/option"
)

func TestLiveAPI(t *testing.T) {
	if os.Getenv("TYPESAFE_API_KEY") == "" {
		t.Skip("TYPESAFE_API_KEY is not set")
	}

	client, err := typesafe.NewClient(
		option.WithMaxRetries(0),
		option.WithTimeout(20*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	t.Run("models", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		models, err := client.Models.List(ctx)
		if err != nil {
			t.Fatalf("Models.List: %v", err)
		}
		if len(models) == 0 {
			t.Fatal("Models.List returned no models")
		}
		for i, model := range models {
			if model.Name == "" {
				t.Fatalf("models[%d] has an empty name", i)
			}
		}
	})

	t.Run("mixed_primitives", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		response, err := client.SystemOne(ctx, typesafe.SystemOneRequest{
			State: "The customer says the export button is broken and asks for help.",
			Questions: typesafe.Questions{
				"intent": typesafe.Choice("What does the customer want?", typesafe.ChoiceCriteria{
					"help":   "Help resolving a problem",
					"praise": "Positive feedback without a problem",
				}),
				"severity":   typesafe.Score("How severe is the reported problem?", "Minor inconvenience", "Feature is unusable"),
				"needs_help": typesafe.Noul("Does the customer ask for help?"),
			},
		})
		if err != nil {
			t.Fatalf("SystemOne: %v", err)
		}
		if response.Model == "" {
			t.Error("response model is empty")
		}
		if response.RequestID == "" {
			t.Error("response request ID is empty")
		}
		if len(response.Answers) != 3 {
			t.Fatalf("got %d answers, want 3", len(response.Answers))
		}

		choice, ok := response.Answers["intent"].(typesafe.ChoiceAnswer)
		if !ok {
			t.Fatalf("intent answer has type %T", response.Answers["intent"])
		}
		if choice.Choice != "help" && choice.Choice != "praise" {
			t.Errorf("intent choice %q is not in the request criteria", choice.Choice)
		}
		checkUnitInterval(t, "intent confidence", choice.Confidence)
		for label, probability := range choice.Probabilities {
			checkUnitInterval(t, "intent probability "+label, probability)
		}

		score, ok := response.Answers["severity"].(typesafe.ScoreAnswer)
		if !ok {
			t.Fatalf("severity answer has type %T", response.Answers["severity"])
		}
		checkUnitInterval(t, "severity score", score.Score)
		checkUnitInterval(t, "severity confidence", score.Confidence)
		if len(score.Legend) != 2 {
			t.Errorf("severity legend has %d levels, want 2", len(score.Legend))
		}
		for level, probability := range score.Probabilities {
			checkUnitInterval(t, "severity probability", probability)
			if level < 0 || level > 1 {
				t.Errorf("severity probability has unexpected level %d", level)
			}
		}

		noul, ok := response.Answers["needs_help"].(typesafe.NoulAnswer)
		if !ok {
			t.Fatalf("needs_help answer has type %T", response.Answers["needs_help"])
		}
		checkUnitInterval(t, "needs_help noul", noul.Noul)

		if response.Usage.InputTokens != nil && *response.Usage.InputTokens < 0 {
			t.Errorf("negative input token count: %d", *response.Usage.InputTokens)
		}
		if response.Usage.OutputTokens != nil && *response.Usage.OutputTokens < 0 {
			t.Errorf("negative output token count: %d", *response.Usage.OutputTokens)
		}
	})
}

func checkUnitInterval(t *testing.T, name string, value float64) {
	t.Helper()
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		t.Errorf("%s = %v, want a finite value in [0, 1]", name, value)
	}
}
