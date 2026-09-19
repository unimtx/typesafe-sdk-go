package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/unimtx/typesafe-sdk-go/option"
)

type conformanceFile struct {
	Schema     int               `json:"schema"`
	UpstreamJS string            `json:"upstream_js"`
	Cases      []conformanceCase `json:"cases"`
}

type conformanceCase struct {
	ID, Source, Method, Path string
	Headers                  map[string]string `json:"headers"`
	Request                  json.RawMessage   `json:"request"`
	Response                 json.RawMessage   `json:"response"`
}

func TestPrimitiveConformanceFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance/primitives.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture conformanceFile
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Schema != 1 || fixture.UpstreamJS != "66880ccded6cb642dc1809620c2b108c33730214" {
		t.Fatal("fixture metadata does not match pinned baseline")
	}
	if len(fixture.Cases) != 25 {
		t.Fatalf("got %d primitive cases, want 25", len(fixture.Cases))
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			request := primitiveRequest(tc.ID)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.Method || r.URL.Path != tc.Path {
					t.Errorf("got %s %s", r.Method, r.URL.Path)
				}
				for name, want := range tc.Headers {
					if got := r.Header.Get(name); got != want {
						t.Errorf("header %s=%q want %q", name, got, want)
					}
				}
				got, _ := io.ReadAll(r.Body)
				assertWireEqual(t, got, tc.Request)
				w.Header().Set("Content-Type", "text/plain") // JSON parsing is content-type independent.
				_, _ = w.Write(tc.Response)
			}))
			defer server.Close()
			client, err := NewClient(option.WithAPIKey("fixture-key"), option.WithBaseURL(server.URL), option.WithMaxRetries(0))
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.SystemOne(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if len(response.Answers) != len(request.Questions) {
				t.Fatalf("decoded %d answers for %d questions", len(response.Answers), len(request.Questions))
			}
			for id, question := range request.Questions {
				answer := response.Answers[id]
				if answer == nil || answer.RawJSON() == "" {
					t.Fatalf("answer %q did not retain raw JSON", id)
				}
				switch question.(type) {
				case NoulQuestion:
					if _, ok := answer.(NoulAnswer); !ok {
						t.Fatalf("answer %q is %T, want NoulAnswer", id, answer)
					}
				case ChoiceQuestion:
					if _, ok := answer.(ChoiceAnswer); !ok {
						t.Fatalf("answer %q is %T, want ChoiceAnswer", id, answer)
					}
				case ScoreQuestion:
					if _, ok := answer.(ScoreAnswer); !ok {
						t.Fatalf("answer %q is %T, want ScoreAnswer", id, answer)
					}
				}
			}
		})
	}
}

// assertWireEqual deliberately ignores top-level member order and Questions key
// order while retaining the encoded order inside State and each question value.
func assertWireEqual(t *testing.T, got, want []byte) {
	t.Helper()
	type envelope struct {
		State     json.RawMessage            `json:"state"`
		Questions map[string]json.RawMessage `json:"questions"`
		Model     json.RawMessage            `json:"model"`
	}
	var a, b envelope
	if err := json.Unmarshal(got, &a); err != nil {
		t.Fatalf("actual wire: %v\n%s", err, got)
	}
	if err := json.Unmarshal(want, &b); err != nil {
		t.Fatalf("expected wire: %v\n%s", err, want)
	}
	compact := func(raw json.RawMessage) []byte {
		var out bytes.Buffer
		if err := json.Compact(&out, raw); err != nil {
			t.Fatal(err)
		}
		return out.Bytes()
	}
	if !bytes.Equal(compact(a.State), compact(b.State)) || !bytes.Equal(compact(a.Model), compact(b.Model)) {
		t.Fatalf("envelope mismatch\n got %s\nwant %s", got, want)
	}
	if len(a.Questions) != len(b.Questions) {
		t.Fatalf("question count %d != %d", len(a.Questions), len(b.Questions))
	}
	for id, wantQuestion := range b.Questions {
		gotQuestion, ok := a.Questions[id]
		if !ok || !bytes.Equal(compact(gotQuestion), compact(wantQuestion)) {
			t.Fatalf("question %q order/shape mismatch\n got %s\nwant %s", id, gotQuestion, wantQuestion)
		}
	}
}

func primitiveRequest(id string) SystemOneRequest {
	switch id {
	case "P01":
		return SystemOneRequest{State: "Please refund this order.", Questions: Questions{"refund_requested": Noul("Does the customer request a refund?")}}
	case "P02":
		return SystemOneRequest{State: map[string]any{"ticket": map[string]any{"subject": "Duplicate charge", "messages": []any{map[string]any{"from": "customer", "text": "Please refund the duplicate."}}}, "order": map[string]any{"id": "A-104", "charges": []any{map[string]any{"amount_usd": 49, "status": "captured"}, map[string]any{"amount_usd": 49, "status": "captured"}}}, "refund_policy": "Duplicate charges are eligible for a refund."}, Questions: Questions{"policy_supports_refund": Noul("Does `refund_policy` support the refund requested in `ticket.messages[0].text`, given `order.charges`?")}}
	case "P03":
		return SystemOneRequest{State: map[string]any{"ticket_message": "My flight was cancelled. Can I get a refund?", "refund_policy": "Cancelled flights are refundable."}, Questions: Questions{"refund_requested": Noul("Does `ticket_message` request a refund?"), "request_type": Choice("What is the main request?", ChoiceCriteria{"refund": "Money returned", "rebooking": "A replacement flight", "information": "Information only"}), "frustration": Score("How frustrated does the customer appear?", "Calm", "Concerned", "Very angry")}}
	case "P03-api-outage":
		return SystemOneRequest{State: "The payments API is down and customers cannot check out.", Questions: Questions{"category": Choice("What kind of incident is this?", ChoiceCriteria{"api": nil, "database": nil, "network": nil}), "outage": Noul("Is this a service outage?"), "severity": Score("How severe is the incident?", "minor", "degraded", "blocking")}}
	case "P04":
		return SystemOneRequest{State: map[string]any{"text": "Account login is broken"}, Questions: Questions{"category": Choice("Which area?", ChoiceCriteria{"account": nil, "billing": nil})}}
	case "C01":
		return SystemOneRequest{State: "My shoes are the wrong size.", Questions: Questions{"department": Choice("Which team should handle this?", ChoiceCriteria{"returns": "Exchanges and refunds", "shipping": "Delivery status", "billing": "Charges and payments"})}}
	case "C01-language":
		return SystemOneRequest{State: "package main", Questions: Questions{"language": Choice("What programming language is this?", ChoiceCriteria{"python": nil, "javascript": nil, "typescript": nil, "go": nil, "rust": nil, "other": nil})}}
	case "C01-meeting":
		return SystemOneRequest{State: "Daily team sync", Questions: Questions{"meeting": Choice("What type of meeting is this?", ChoiceCriteria{"standup": nil, "planning": nil, "retrospective": nil, "one_on_one": nil, "brainstorm": nil, "none": nil})}}
	case "C01-category":
		return SystemOneRequest{State: "Cotton shirt", Questions: Questions{"category": Choice("Which product category?", ChoiceCriteria{"electronics": nil, "clothing": nil, "home_garden": nil, "food_beverage": nil})}}
	case "C02":
		return SystemOneRequest{State: "The wrong-size item was late and charged twice.", Questions: Questions{
			"department":           Choice("Which team?", ChoiceCriteria{"returns": "Returns", "shipping": "Delivery", "billing": "Payments"}),
			"return_reason":        Choice("Why return?", ChoiceCriteria{"wrong_size": "Does not fit", "wrong_item": "Different item", "damaged": "Broken", "other": "Other"}),
			"shipping_issue":       Choice("Which shipping issue?", ChoiceCriteria{"not_delivered": "Missing", "delayed": "Late", "wrong_address": "Wrong place", "other": "Other"}),
			"requested_resolution": Choice("What should happen?", ChoiceCriteria{"exchange": "Swap", "refund": "Money back", "replacement": "Send again", "information": "Answer"}),
			"tone":                 Choice("What tone?", ChoiceCriteria{"calm": nil, "frustrated": nil, "angry": nil}),
		}}
	case "C03":
		return SystemOneRequest{State: map[string]any{"order": map[string]any{"status": "delivered"}, "policy": map[string]any{"days": 30}}, Questions: Questions{"intent": ChoiceQuestion{Instructions: map[string]any{"task": "Classify request", "compare": []string{"order.status", "policy.days"}}, Criteria: ChoiceCriteria{"return_policy": map[string]any{"covers": []string{"eligibility", "rules"}}, "return_status": map[string]any{"covers": []string{"tracking", "progress"}}}}}}
	case "S01":
		return SystemOneRequest{State: "Export crashes Safari but works in Chrome.", Questions: Questions{"bug_severity": Score("How severe is the reported issue?", "Cosmetic; no impact", "Broken but workaround exists", "Blocking; no workaround")}}
	case "S02":
		return SystemOneRequest{State: "The spinner never finishes; CSV may work.", Questions: Questions{"severity": Score("Rate severity.", "0", "1", "2")}}
	case "S02-low":
		return SystemOneRequest{State: "The export button is misaligned by a few pixels.", Questions: Questions{"severity": Score("How severe is the reported issue?", "Cosmetic", "Workaround exists", "Blocking")}}
	case "S02-middle":
		return SystemOneRequest{State: "PDF export fails but CSV is a workaround.", Questions: Questions{"severity": Score("How severe is the reported issue?", "Cosmetic", "Workaround exists", "Blocking")}}
	case "S02-high":
		return SystemOneRequest{State: "Nobody can log in; every attempt returns 500.", Questions: Questions{"severity": Score("How severe is the reported issue?", "Cosmetic", "Workaround exists", "Blocking")}}
	case "S03":
		return SystemOneRequest{State: "Export fails for the third time. Safari 18, click Export, spinner hangs.", Questions: Questions{"severity": Score("How severe?", "Cosmetic", "Workaround exists", "Blocking"), "frustration": Score("How frustrated?", "Calm", "Frustrated", "Very angry"), "report_quality": Score("How useful is the report?", "No detail", "Names feature", "Steps or environment", "Steps and environment")}}
	case "S04":
		return SystemOneRequest{State: map[string]any{"change": map[string]any{"files": 8, "services": []string{"api", "worker"}}}, Questions: Questions{"scope": ScoreQuestion{Instructions: map[string]any{"task": "Evaluate change scope"}, Criteria: []Entry{map[string]any{"impact": "local", "signals": []string{"one file"}}, map[string]any{"impact": "moderate", "signals": []string{"several files"}}, map[string]any{"impact": "broad", "signals": []string{"multiple services"}}}}}}
	case "N01":
		return SystemOneRequest{State: "I already contacted support. Please get a human.", Questions: Questions{"is_human_escalation": Noul("Does the customer request a human?"), "is_repeat_contact": NoulQuestion{Instructions: "Has the customer contacted support before?", Criteria: &NoulCriteria{True: "Mentions an earlier contact", False: "No earlier contact"}}}}
	case "A01":
		return SystemOneRequest{State: map[string]any{"invoice": map[string]any{"amount": 120, "currency": "USD"}}, Questions: Questions{"valid": Noul(map[string]any{"task": "Verify invoice", "required": []string{"amount", "currency"}}), "category": Choice(map[string]any{"task": "Classify currency", "field": "invoice.currency"}, ChoiceCriteria{"supported": nil, "unsupported": nil}), "risk": Score(map[string]any{"task": "Rate risk", "field": "invoice.amount"}, "low", "high"), "urgency": Score(map[string]any{"task": "Rate urgency"}, "later", "now")}}
	case "A02":
		return SystemOneRequest{State: map[string]any{"actual": map[string]any{"total": 10}, "expected": map[string]any{"total": 10}}, Questions: Questions{"matches": Noul(map[string]any{"task": "Compare fields", "fields": []string{"actual.total", "expected.total"}}), "array_form": Noul([]string{"actual.total", "expected.total"})}}
	case "A03":
		return SystemOneRequest{State: "Where is order A-1?", Questions: Questions{"team": Choice("Which team?", ChoiceCriteria{"billing": map[string]any{"scope": "Payments", "examples": []string{"charge", "invoice"}}, "orders": map[string]any{"scope": "Orders", "examples": []string{"tracking", "return"}}, "account": map[string]any{"scope": "Accounts", "examples": []string{"login", "profile"}}})}}
	case "A04":
		return SystemOneRequest{State: "Cannot reset my password.", Questions: Questions{"category": Choice("Choose top-level category.", ChoiceCriteria{"account": map[string]any{"children": []string{"login", "profile"}}, "orders": map[string]any{"children": []string{"delivery", "returns"}}})}}
	case "A05":
		return SystemOneRequest{State: map[string]any{"change": map[string]any{"files": []string{"api.go", "worker.go"}}}, Questions: Questions{"scope": ScoreQuestion{Instructions: map[string]any{"task": "Evaluate scope"}, Criteria: []Entry{map[string]any{"label": "small", "signals": []string{"one component"}}, map[string]any{"label": "large", "signals": []string{"several components"}}}}}}
	case "A06":
		return SystemOneRequest{State: map[string]any{"message": "Send me your password."}, Questions: Questions{"credential_request": NoulQuestion{Instructions: map[string]any{"task": "Detect credential request", "field": "message"}, Criteria: &NoulCriteria{True: map[string]any{"meaning": "Requests a secret", "examples": []string{"password", "token"}}, False: map[string]any{"meaning": "Does not request a secret"}}}}}
	default:
		panic(fmt.Sprintf("unknown fixture %q", id))
	}
}

func TestDependentAndApplicationWorkflows(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if calls == 1 {
			if !bytes.Contains(body, []byte(`"account"`)) {
				t.Error("first taxonomy level missing")
			}
			_, _ = io.WriteString(w, `{"model":"m","answers":{"category":{"type":"choice","choice":"account","confidence":1,"probabilities":{"account":1,"orders":0}}},"usage":{}}`)
			return
		}
		if !bytes.Contains(body, []byte(`"login"`)) || !bytes.Contains(body, []byte(`"profile"`)) {
			t.Error("selected children missing from dependent request")
		}
		_, _ = io.WriteString(w, `{"model":"m","answers":{"subcategory":{"type":"choice","choice":"login","confidence":1,"probabilities":{"login":1,"profile":0}}},"usage":{}}`)
	}))
	defer server.Close()
	client, _ := NewClient(option.WithAPIKey("k"), option.WithBaseURL(server.URL), option.WithMaxRetries(0))
	first, err := client.SystemOne(context.Background(), primitiveRequest("A04"))
	if err != nil {
		t.Fatal(err)
	}
	selected := first.Choices()["category"].Choice
	children := map[string][]string{"account": {"login", "profile"}, "orders": {"delivery", "returns"}}[selected]
	criteria := make(ChoiceCriteria, len(children))
	for _, child := range children {
		criteria[child] = nil
	}
	second, err := client.SystemOne(context.Background(), SystemOneRequest{State: "Cannot reset my password.", Questions: Questions{"subcategory": Choice("Choose subcategory.", criteria)}})
	if err != nil || second.Choices()["subcategory"].Choice != "login" || calls != 2 {
		t.Fatalf("dependent workflow: calls=%d response=%v err=%v", calls, second, err)
	}

	answers := map[string]ScoreAnswer{"severity": {Score: 1.24}, "frustration": {Score: 1.45}, "report_quality": {Score: 3}}
	criteriaLengths := map[string]int{"severity": 3, "frustration": 3, "report_quality": 4}
	priority := .6*(answers["severity"].Score/float64(criteriaLengths["severity"]-1)) + .3*(answers["frustration"].Score/float64(criteriaLengths["frustration"]-1)) + .1*(answers["report_quality"].Score/float64(criteriaLengths["report_quality"]-1))
	if mathAbs(priority-.6895) > 1e-9 {
		t.Fatalf("composite priority=%v", priority)
	}
}

func TestRawQuestionPreservesMemberOrder(t *testing.T) {
	q := RawQuestion{JSON: json.RawMessage(`{"type":"future","z":1,"a":2}`)}
	body, err := json.Marshal(struct {
		Questions Questions `json:"questions"`
	}{Questions{"q": q}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`{"type":"future","z":1,"a":2}`)) {
		t.Fatalf("raw member order changed: %s", body)
	}
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
