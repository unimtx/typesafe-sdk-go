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
		State: "My running shoes arrived in the wrong size. Can I swap them for a size 10?",
		Questions: typesafe.Questions{
			"department": typesafe.Choice(
				"Which team should handle this?",
				typesafe.ChoiceCriteria{
					"returns":  "Exchanges, refunds, wrong or damaged items",
					"shipping": "Delivery status, delays, lost packages",
					"billing":  "Charges, invoices, payment problems",
				},
			),
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	answer, ok := response.Choices()["department"]
	if !ok {
		log.Fatal("response did not contain a Choice answer for department")
	}
	fmt.Printf("choice=%s confidence=%.2f probabilities=%v\n",
		answer.Choice, answer.Confidence, answer.Probabilities)
}
