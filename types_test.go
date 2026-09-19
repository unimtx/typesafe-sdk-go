package typesafe

import (
	"encoding/json"
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
