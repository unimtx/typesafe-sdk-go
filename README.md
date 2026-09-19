# TypeSafe SDK for Go

A **community-maintained Go SDK** for the TypeSafe HTTP API, built to fill the
gap left by the absence of an official Go SDK and enable quick integration with
Jev, a System One model. This project is unaffiliated with and not endorsed by TypeSafe.

The API favors normal Go conventions: contexts, keyed structs, scoped functional
options, concrete errors, and standard `net/http` interoperability. Runtime code
uses only the Go standard library.

## Install

```sh
go get github.com/unimtx/typesafe-sdk-go
```

The minimum supported version is Go 1.24.0.

## Quickstart

Set `TYPESAFE_API_KEY` in your environment, then create and use the client:

```sh
export TYPESAFE_API_KEY="your-api-key"
```

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/unimtx/typesafe-sdk-go"
)

func main() {
	ctx := context.Background()
	client, err := typesafe.NewClient() // reads TYPESAFE_API_KEY
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.SystemOne(ctx, typesafe.SystemOneRequest{
		State: map[string]any{"document": "I was charged twice. Please fix this ASAP."},
		Questions: typesafe.Questions{
			"billing": typesafe.Noul("Is this ticket about billing?"),
			"tone": typesafe.Choice("What is the customer's tone?", typesafe.ChoiceCriteria{
				"calm": nil, "frustrated": nil, "angry": nil,
			}),
			"urgency": typesafe.Score("How urgent is this ticket?", "can wait", "this week", "today"),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	if tone, ok := resp.Choices()["tone"]; ok {
		fmt.Println(tone.Choice, tone.Confidence)
	}
}
```

Question IDs are wire keys and response lookup keys, but the model does not use
them for inference. Put the complete judgment in `Instructions`. State must be a
JSON string, object, or array. Instructions and criteria entries may also be
structured values or null.

## Core primitives

TypeSafe provides three question types. Questions in one request share the same
state, are evaluated independently, and return typed answers under the IDs you
choose.

### Choice

Use Choice to select one option from a fixed set. This example routes a shoe
store support ticket to the team that should handle it:

```go
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
```

`Choice` is the selected criterion. `Probabilities` contains the complete
distribution across the supplied options, and `Confidence` summarizes how
concentrated that distribution is.

### Score

Use Score to rate the state along ordered, descriptive levels. This example
rates a browser-specific export failure by severity:

```go
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
```

Criteria are ordered from the low end of the scale to the high end. `Score` is
a fractional position along those levels, not a rounded level number.
`Probabilities` and `Confidence` show how the result is distributed.

### Noul

Use Noul for a yes/no judgment when the probability of yes is useful. This
example checks whether a customer requests a human and whether this is a repeat
contact. The second question clarifies what true and false mean:

```go
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

	answers := response.Nouls()
	humanEscalation, humanOK := answers["is_human_escalation"]
	repeatContact, repeatOK := answers["is_repeat_contact"]
	if !humanOK || !repeatOK {
		log.Fatal("response did not contain both Noul answers")
	}
	fmt.Printf("human escalation=%.2f repeat contact=%.2f\n",
		humanEscalation.Noul,
		repeatContact.Noul)
	if humanEscalation.Noul >= 0.8 {
		fmt.Println("route to a human")
	}
}
```

`Noul` ranges from zero to one and represents the probability that the answer
is yes. It has no separate confidence value. The application chooses a threshold
when it needs a boolean decision.

The same programs are available under [`examples`](examples):

```sh
go run ./examples/choice
go run ./examples/score
go run ./examples/noul
```

Running these examples calls the live TypeSafe API and may incur usage. The
repository's ordinary test suite remains offline.

## Error handling

SDK errors work with the standard `errors` package:

```go
var apiErr *typesafe.APIError
switch {
case errors.Is(err, typesafe.ErrRateLimit):
	log.Print("rate limited; try again later")
case errors.As(err, &apiErr):
	log.Printf("HTTP %d (request %s)", apiErr.StatusCode, apiErr.RequestID)
case err != nil:
	log.Print(err)
}
```

Transient failures are retried by default. If a response is lost, a retry may
repeat an evaluation and incur duplicate usage.

## License

MIT. See [LICENSE](LICENSE) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
