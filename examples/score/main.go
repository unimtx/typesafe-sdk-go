package main

import (
	"context"
	"fmt"
	"log"

	"github.com/unimtx/typesafe-sdk-go"
)

func main() {
	client, err := typesafe.NewClient() // reads TYPESAFE_API_KEY
	if err != nil {
		log.Fatal(err)
	}

	response, err := client.SystemOne(context.Background(), typesafe.SystemOneRequest{
		State: "The export button crashes the settings page in Safari. It works in Chrome, but a few of our customers only use Safari.",
		Questions: typesafe.Questions{
			"bug_severity": typesafe.Score(
				"How severe is the reported issue?",
				"Cosmetic; no impact to functionality",
				"Broken or degraded feature, but workaround exists",
				"Blocking issue; no workaround exists",
			),
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	answer, ok := response.Scores()["bug_severity"]
	if !ok {
		log.Fatal("response did not contain a Score answer for bug_severity")
	}
	fmt.Printf("score=%.2f confidence=%.2f probabilities=%v\n",
		answer.Score, answer.Confidence, answer.Probabilities)
}
