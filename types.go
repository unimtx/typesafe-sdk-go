package typesafe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// Entry is a JSON-compatible state, instruction, or description value.
// Runtime request validation enforces the narrower documented root State shape.
type Entry = any

// SystemOneRequest contains shared state and independently evaluated questions.
// An empty Model inherits the Client default; the transmitted body always includes
// the resolved model.
type SystemOneRequest struct {
	// State is the shared JSON string, object, or array to evaluate.
	State Entry `json:"state"`
	// Questions are independent judgments keyed by response ID.
	Questions Questions `json:"questions"`
	// Model overrides the client default when nonempty.
	Model string `json:"model,omitzero"`
}

// Questions maps wire-visible question IDs to typed questions.
type Questions map[string]Question

// Question is implemented by the SDK's question types. It is sealed so a
// foreign value cannot accidentally become a wire question.
type Question interface{ isQuestion() }

// ChoiceCriteria maps option names to descriptions. Values may be structured or nil.
type ChoiceCriteria map[string]Entry

// ChoiceQuestion asks the model to select one named criterion.
type ChoiceQuestion struct {
	// Instructions describe the judgment and always serialize, including as null.
	Instructions Entry `json:"instructions"`
	// Criteria maps every selectable name to its optional description.
	Criteria ChoiceCriteria `json:"criteria"`
}

func (ChoiceQuestion) isQuestion() {}

// MarshalJSON adds the fixed choice discriminator and normalizes nil criteria to {}.
func (q ChoiceQuestion) MarshalJSON() ([]byte, error) {
	criteria := q.Criteria
	if criteria == nil {
		criteria = ChoiceCriteria{}
	}
	return json.Marshal(struct {
		Type         string         `json:"type"`
		Instructions Entry          `json:"instructions"`
		Criteria     ChoiceCriteria `json:"criteria"`
	}{"choice", q.Instructions, criteria})
}

// ScoreQuestion asks for a fractional expected position on an ordered rubric.
type ScoreQuestion struct {
	// Instructions describe the judgment and always serialize, including as null.
	Instructions Entry `json:"instructions"`
	// Criteria are ordered low-to-high levels; SystemOne requires at least two.
	Criteria []Entry `json:"criteria"`
}

func (ScoreQuestion) isQuestion() {}

// MarshalJSON adds the fixed score discriminator. Call-time validation requires
// at least two criteria; direct marshaling intentionally does not.
func (q ScoreQuestion) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type         string  `json:"type"`
		Instructions Entry   `json:"instructions"`
		Criteria     []Entry `json:"criteria"`
	}{"score", q.Instructions, q.Criteria})
}

// NoulCriteria optionally describes the true and false outcomes.
// Nil fields are omitted; use json.RawMessage("null") for an explicit null field.
type NoulCriteria struct {
	// True describes what a yes outcome means.
	True Entry `json:"true,omitzero"`
	// False describes what a no outcome means.
	False Entry `json:"false,omitzero"`
}

// NoulQuestion asks for the probability of yes. A nil Criteria pointer omits
// the criteria object; a non-nil zero value sends an empty object.
type NoulQuestion struct {
	// Instructions describe the yes/no judgment and always serialize, including as null.
	Instructions Entry `json:"instructions"`
	// Criteria optionally clarifies either outcome; nil omits the whole object.
	Criteria *NoulCriteria `json:"criteria,omitzero"`
}

func (NoulQuestion) isQuestion() {}

// MarshalJSON adds the fixed noul discriminator.
func (q NoulQuestion) MarshalJSON() ([]byte, error) {
	if q.Criteria == nil {
		return json.Marshal(struct {
			Type         string `json:"type"`
			Instructions Entry  `json:"instructions"`
		}{"noul", q.Instructions})
	}
	return json.Marshal(struct {
		Type         string        `json:"type"`
		Instructions Entry         `json:"instructions"`
		Criteria     *NoulCriteria `json:"criteria"`
	}{"noul", q.Instructions, q.Criteria})
}

// RawQuestion forwards a complete future-compatible question object.
// SystemOne validates that JSON is an object with a nonempty string type.
type RawQuestion struct {
	// JSON is a complete question object with a nonempty string type.
	JSON json.RawMessage
}

func (RawQuestion) isQuestion() {}

// MarshalJSON returns the raw question JSON through encoding/json validation.
func (q RawQuestion) MarshalJSON() ([]byte, error) {
	if len(q.JSON) == 0 {
		return nil, fmt.Errorf("raw question JSON is empty")
	}
	return q.JSON.MarshalJSON()
}

// Choice builds a choice question and shallow-copies criteria. Nested values remain shared.
func Choice(instructions Entry, criteria ChoiceCriteria) ChoiceQuestion {
	var copied ChoiceCriteria
	if criteria != nil {
		copied = make(ChoiceCriteria, len(criteria))
		for name, description := range criteria {
			copied[name] = description
		}
	}
	return ChoiceQuestion{Instructions: instructions, Criteria: copied}
}

// Score builds a score question and copies the outer ordered level slice.
// Use Score[Entry] for mixed concrete values, all nil values, or no levels.
func Score[T any](instructions Entry, levels ...T) ScoreQuestion {
	var criteria []Entry
	if levels != nil {
		criteria = make([]Entry, len(levels))
		for i := range levels {
			criteria[i] = levels[i]
		}
	}
	return ScoreQuestion{Instructions: instructions, Criteria: criteria}
}

// Noul builds a yes/no probability question without outcome descriptions.
func Noul(instructions Entry) NoulQuestion { return NoulQuestion{Instructions: instructions} }

// Usage reports token counts. Nil means the server omitted the value or sent null;
// a pointer to zero is a reported zero.
type Usage struct {
	// InputTokens is nil when omitted or null and points to a reported count otherwise.
	InputTokens *int `json:"input_tokens,omitzero"`
	// OutputTokens is nil when omitted or null and points to a reported count otherwise.
	OutputTokens *int `json:"output_tokens,omitzero"`
}

// Answer is a decoded response answer with access to its original received JSON.
type Answer interface {
	isAnswer()
	RawJSON() string
}

// NoulAnswer contains the probability of yes, not a boolean decision.
type NoulAnswer struct {
	// Noul is the probability of yes, from zero to one under the API contract.
	Noul float64 `json:"noul"`
	raw  []byte
}

func (NoulAnswer) isAnswer() {}

// RawJSON returns the original received answer JSON or "" for a manually built value.
func (a NoulAnswer) RawJSON() string { return string(a.raw) }

// ChoiceAnswer contains a selected option and the complete probability distribution.
type ChoiceAnswer struct {
	// Choice is the selected criterion name.
	Choice string `json:"choice"`
	// Confidence summarizes how concentrated the probability distribution is.
	Confidence float64 `json:"confidence"`
	// Probabilities contains the distribution keyed by criterion name.
	Probabilities map[string]float64 `json:"probabilities"`
	raw           []byte
}

func (ChoiceAnswer) isAnswer() {}

// RawJSON returns the original received answer JSON or "" for a manually built value.
func (a ChoiceAnswer) RawJSON() string { return string(a.raw) }

// ScoreAnswer contains a fractional expected level and integer-keyed rubric data.
type ScoreAnswer struct {
	// Score is the fractional expected position on the supplied level scale.
	Score float64 `json:"score"`
	// Confidence summarizes how concentrated the probability distribution is.
	Confidence float64 `json:"confidence"`
	// Legend maps integer level positions to their descriptions.
	Legend map[int]Entry `json:"legend"`
	// Probabilities maps integer level positions to probabilities.
	Probabilities map[int]float64 `json:"probabilities"`
	raw           []byte
}

func (ScoreAnswer) isAnswer() {}

// RawJSON returns the original received answer JSON or "" for a manually built value.
func (a ScoreAnswer) RawJSON() string { return string(a.raw) }

// UnknownAnswer preserves a future answer kind with a nonempty discriminator.
type UnknownAnswer struct {
	// Type is the future, nonempty wire discriminator.
	Type string `json:"type"`
	raw  []byte
}

func (UnknownAnswer) isAnswer() {}

// RawJSON returns the original received answer JSON or "" for a manually built value.
func (a UnknownAnswer) RawJSON() string { return string(a.raw) }

// SystemOneResponse contains decoded answers, usage, and the response request ID.
type SystemOneResponse struct {
	// Model is the model reported by the service.
	Model string `json:"model"`
	// Answers are keyed by the IDs supplied in the request.
	Answers map[string]Answer `json:"-"`
	// Usage contains optional reported token counts.
	Usage Usage `json:"usage"`
	// RequestID is copied from the response header and is not part of JSON.
	RequestID string `json:"-"`
	raw       []byte
}

// RawJSON returns an owned snapshot of the original received response body.
func (r *SystemOneResponse) RawJSON() string {
	if r == nil {
		return ""
	}
	return string(r.raw)
}

// Nouls returns a new map containing noul answer values. Nested decoded values
// remain shared with the response.
func (r *SystemOneResponse) Nouls() map[string]NoulAnswer {
	out := make(map[string]NoulAnswer)
	if r != nil {
		for id, answer := range r.Answers {
			if v, ok := answer.(NoulAnswer); ok {
				out[id] = v
			}
		}
	}
	return out
}

// Choices returns a new map containing choice answer values. Nested probability
// maps remain shared with the response.
func (r *SystemOneResponse) Choices() map[string]ChoiceAnswer {
	out := make(map[string]ChoiceAnswer)
	if r != nil {
		for id, answer := range r.Answers {
			if v, ok := answer.(ChoiceAnswer); ok {
				out[id] = v
			}
		}
	}
	return out
}

// Scores returns a new map containing score answer values. Nested legend and
// probability maps remain shared with the response.
func (r *SystemOneResponse) Scores() map[string]ScoreAnswer {
	out := make(map[string]ScoreAnswer)
	if r != nil {
		for id, answer := range r.Answers {
			if v, ok := answer.(ScoreAnswer); ok {
				out[id] = v
			}
		}
	}
	return out
}

// ModelCard describes a model returned by Models.List. ReleaseDate remains the wire string.
type ModelCard struct {
	// Name is the model identifier.
	Name string `json:"name"`
	// Description is the service-provided model description.
	Description string `json:"description"`
	// ReleaseDate is the service-provided date string.
	ReleaseDate string `json:"release_date"`
}

func decodeSystemOne(data []byte, requestID string) (*SystemOneResponse, error) {
	var envelope struct {
		Model   string                     `json:"model"`
		Answers map[string]json.RawMessage `json:"answers"`
		Usage   Usage                      `json:"usage"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	answers := make(map[string]Answer, len(envelope.Answers))
	for id, raw := range envelope.Answers {
		answer, err := decodeAnswer(raw)
		if err != nil {
			return nil, fmt.Errorf("answer %q: %w", id, err)
		}
		answers[id] = answer
	}
	return &SystemOneResponse{Model: envelope.Model, Answers: answers, Usage: envelope.Usage,
		RequestID: requestID, raw: bytes.Clone(data)}, nil
}

func decodeAnswer(raw json.RawMessage) (Answer, error) {
	var discriminator struct {
		Type json.RawMessage `json:"type"`
	}
	if err := json.Unmarshal(raw, &discriminator); err != nil {
		return nil, err
	}
	var kind string
	if len(discriminator.Type) == 0 || json.Unmarshal(discriminator.Type, &kind) != nil || kind == "" {
		return nil, fmt.Errorf("answer has a missing, invalid, or empty type")
	}
	owned := bytes.Clone(raw)
	switch kind {
	case "noul":
		var wire struct {
			Noul float64 `json:"noul"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			return nil, err
		}
		return NoulAnswer{Noul: wire.Noul, raw: owned}, nil
	case "choice":
		var wire struct {
			Choice        string             `json:"choice"`
			Confidence    float64            `json:"confidence"`
			Probabilities map[string]float64 `json:"probabilities"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			return nil, err
		}
		return ChoiceAnswer{Choice: wire.Choice, Confidence: wire.Confidence, Probabilities: wire.Probabilities, raw: owned}, nil
	case "score":
		var wire struct {
			Score         float64            `json:"score"`
			Confidence    float64            `json:"confidence"`
			Legend        map[string]Entry   `json:"legend"`
			Probabilities map[string]float64 `json:"probabilities"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			return nil, err
		}
		legend := make(map[int]Entry, len(wire.Legend))
		probabilities := make(map[int]float64, len(wire.Probabilities))
		for key, value := range wire.Legend {
			n, err := strconv.Atoi(key)
			if err != nil {
				return nil, fmt.Errorf("legend key %q is not an integer", key)
			}
			legend[n] = value
		}
		for key, value := range wire.Probabilities {
			n, err := strconv.Atoi(key)
			if err != nil {
				return nil, fmt.Errorf("probability key %q is not an integer", key)
			}
			probabilities[n] = value
		}
		return ScoreAnswer{Score: wire.Score, Confidence: wire.Confidence, Legend: legend, Probabilities: probabilities, raw: owned}, nil
	default:
		return UnknownAnswer{Type: kind, raw: owned}, nil
	}
}
