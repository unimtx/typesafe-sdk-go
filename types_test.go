package typesafe

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestQuestionMarshalAndOwnership(t *testing.T) {
	criteria := ChoiceCriteria{"10": nil, "2": "two", "a": map[string]any{"nested": true}}
	q := Choice("choose", criteria)
	criteria["2"] = "changed"
	got, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"choice","instructions":"choose","criteria":{"10":null,"2":"two","a":{"nested":true}}}`
	if string(got) != want {
		t.Fatalf("choice JSON\n got %s\nwant %s", got, want)
	}

	nilChoice, _ := json.Marshal(Choice("x", nil))
	if string(nilChoice) != `{"type":"choice","instructions":"x","criteria":{}}` {
		t.Fatalf("nil choice: %s", nilChoice)
	}

	levels := []string{"low", "high"}
	score := Score("rank", levels...)
	levels[0] = "changed"
	if score.Criteria[0] != "low" {
		t.Fatal("Score did not copy the outer slice")
	}
	if Score[Entry]("rank").Criteria != nil {
		t.Fatal("no-level Score should preserve nil")
	}

	noul, _ := json.Marshal(NoulQuestion{Instructions: nil, Criteria: &NoulCriteria{True: "", False: json.RawMessage("null")}})
	if string(noul) != `{"type":"noul","instructions":null,"criteria":{"true":"","false":null}}` {
		t.Fatalf("noul JSON: %s", noul)
	}
	empty, _ := json.Marshal(NoulQuestion{Instructions: "?", Criteria: &NoulCriteria{}})
	if string(empty) != `{"type":"noul","instructions":"?","criteria":{}}` {
		t.Fatalf("empty criteria: %s", empty)
	}
}

func TestDecodeAnswersAndRawJSON(t *testing.T) {
	raw := []byte(`{"model":"m","answers":{"n":{"type":"noul","noul":0.75},"c":{"type":"choice","choice":"a","confidence":0.8,"probabilities":{"a":0.8}},"s":{"type":"score","score":1.25,"confidence":0.6,"legend":{"0":null,"1":{"x":true}},"probabilities":{"0":0.25,"1":0.75}},"u":{"type":"future","value":1}},"usage":{"input_tokens":0,"output_tokens":null}}`)
	response, err := decodeSystemOne(raw, "req_1")
	if err != nil {
		t.Fatal(err)
	}
	if response.RequestID != "req_1" || response.RawJSON() != string(raw) {
		t.Fatal("response metadata/raw mismatch")
	}
	if got := response.Nouls()["n"].Noul; got != .75 {
		t.Fatalf("noul=%v", got)
	}
	if got := response.Scores()["s"].Score; got != 1.25 {
		t.Fatalf("score=%v", got)
	}
	if _, ok := response.Answers["u"].(UnknownAnswer); !ok {
		t.Fatalf("unknown type %T", response.Answers["u"])
	}
	if response.Usage.InputTokens == nil || *response.Usage.InputTokens != 0 || response.Usage.OutputTokens != nil {
		t.Fatal("usage absence/zero mismatch")
	}
	before := response.Answers["c"].RawJSON()
	choice := response.Answers["c"].(ChoiceAnswer)
	choice.Choice = "changed"
	if response.Answers["c"].RawJSON() != before {
		t.Fatal("raw JSON changed with field edit")
	}
	copyMap := response.Choices()
	delete(copyMap, "c")
	if _, ok := response.Answers["c"]; !ok {
		t.Fatal("typed accessor exposed original map")
	}
}

func TestSystemOneResponseValidationPaths(t *testing.T) {
	tests := []struct {
		name string
		body string
		path string
	}{
		{"root type", `[]`, "$"},
		{"missing model", `{"answers":{},"usage":{}}`, "/model"},
		{"null model", `{"model":null,"answers":{},"usage":{}}`, "/model"},
		{"empty model", `{"model":"","answers":{},"usage":{}}`, "/model"},
		{"null answers", `{"model":"m","answers":null,"usage":{}}`, "/answers"},
		{"missing usage", `{"model":"m","answers":{}}`, "/usage"},
		{"invalid usage count", `{"model":"m","answers":{},"usage":{"input_tokens":"one"}}`, "/usage/input_tokens"},
		{"missing answer type", `{"model":"m","answers":{"a/b":{"noul":1}},"usage":{}}`, "/answers/a~1b/type"},
		{"missing noul", `{"model":"m","answers":{"q":{"type":"noul"}},"usage":{}}`, "/answers/q/noul"},
		{"missing choice probabilities", `{"model":"m","answers":{"q":{"type":"choice","choice":"a","confidence":1}},"usage":{}}`, "/answers/q/probabilities"},
		{"null score legend", `{"model":"m","answers":{"q":{"type":"score","score":1,"confidence":1,"legend":null,"probabilities":{}}},"usage":{}}`, "/answers/q/legend"},
		{"noninteger score key", `{"model":"m","answers":{"q":{"type":"score","score":1,"confidence":1,"legend":{"low":"x"},"probabilities":{}}},"usage":{}}`, "/answers/q/legend/low"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeSystemOne([]byte(test.body), "request")
			var fieldErr *responseFieldError
			if !errors.As(err, &fieldErr) || fieldErr.path != test.path {
				t.Fatalf("error=%v path=%q, want %q", err, fieldErrPath(err), test.path)
			}
		})
	}
}

func TestModelsResponseValidationPaths(t *testing.T) {
	tests := []struct {
		body string
		path string
	}{
		{`null`, "$"},
		{`{}`, "/models"},
		{`{"models":null}`, "/models"},
		{`{"models":[null]}`, "/models/0"},
		{`{"models":[{"name":"m","description":"d"}]}`, "/models/0/release_date"},
		{`{"models":[{"name":1,"description":"d","release_date":"2026"}]}`, "/models/0/name"},
	}
	for _, test := range tests {
		_, err := decodeModels([]byte(test.body))
		var fieldErr *responseFieldError
		if !errors.As(err, &fieldErr) || fieldErr.path != test.path {
			t.Errorf("body=%s error=%v path=%q, want %q", test.body, err, fieldErrPath(err), test.path)
		}
	}
}

func fieldErrPath(err error) string {
	var fieldErr *responseFieldError
	if errors.As(err, &fieldErr) {
		return fieldErr.path
	}
	return ""
}

func TestValidateAnswerKinds(t *testing.T) {
	tests := []struct {
		name      string
		answers   map[string]Answer
		questions Questions
		path      string
	}{
		{"missing", map[string]Answer{}, Questions{"q": Noul("?")}, "/answers/q"},
		{"mismatch", map[string]Answer{"q": NoulAnswer{}}, Questions{"q": Choice("?", ChoiceCriteria{"a": nil})}, "/answers/q/type"},
		{"escaped ID", map[string]Answer{}, Questions{"a/b": Noul("?")}, "/answers/a~1b"},
	}
	for _, test := range tests {
		err := validateAnswerKinds(test.answers, test.questions)
		if got := fieldErrPath(err); got != test.path {
			t.Errorf("%s: error=%v path=%q, want %q", test.name, err, got, test.path)
		}
	}
	if err := validateAnswerKinds(
		map[string]Answer{"future": UnknownAnswer{Type: "future"}},
		Questions{"future": RawQuestion{JSON: json.RawMessage(`{"type":"future"}`)}},
	); err != nil {
		t.Fatalf("matching future types: %v", err)
	}
}

func TestDecodeAnswerRejectsInvalidDiscriminatorsAndScoreKeys(t *testing.T) {
	for _, raw := range []string{`{}`, `{"type":null}`, `{"type":""}`, `{"type":"score","legend":{"x":"bad"},"probabilities":{}}`} {
		if _, err := decodeAnswer([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, err := decodeAnswer([]byte(`{"type":"choice","confidence":"bad"}`)); err == nil {
		t.Fatal("accepted invalid known field")
	}
}

func TestHelpersEqualLiterals(t *testing.T) {
	cases := [][2]Question{
		{Choice("q", ChoiceCriteria{"a": nil}), ChoiceQuestion{Instructions: "q", Criteria: ChoiceCriteria{"a": nil}}},
		{Score("q", "a", "b"), ScoreQuestion{Instructions: "q", Criteria: []Entry{"a", "b"}}},
		{Noul("q"), NoulQuestion{Instructions: "q"}},
	}
	for _, pair := range cases {
		a, _ := json.Marshal(pair[0])
		b, _ := json.Marshal(pair[1])
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("%s != %s", a, b)
		}
	}
}
