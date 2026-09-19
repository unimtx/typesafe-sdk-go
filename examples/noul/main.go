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
		State: "I already contacted support. Please get a human.",
		Questions: typesafe.Questions{
			"is_human_escalation": typesafe.Noul("Does the customer request a human?"),
			"is_repeat_contact": typesafe.NoulQuestion{
				Instructions: "Has the customer contacted support before?",
				Criteria: &typesafe.NoulCriteria{
					True:  "Mentions an earlier contact",
					False: "No earlier contact",
				},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	humanEscalation, err := response.Noul("is_human_escalation")
	if err != nil {
		log.Fatal(err)
	}
	repeatContact, err := response.Noul("is_repeat_contact")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("human escalation=%.2f repeat contact=%.2f\n",
		humanEscalation.Noul,
		repeatContact.Noul)
	if humanEscalation.Noul >= 0.8 {
		fmt.Println("route to a human")
	}
}
