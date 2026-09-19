package typesafe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
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

type responseFieldError struct {
	path  string
	cause error
}

func (e *responseFieldError) Error() string { return e.cause.Error() }
func (e *responseFieldError) Unwrap() error { return e.cause }

func invalidResponseField(path, format string, args ...any) error {
	return &responseFieldError{path: path, cause: fmt.Errorf(format, args...)}
}

func responseObject(raw []byte, path string) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, invalidResponseField(path, "expected a JSON object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return nil, &responseFieldError{path: path, cause: err}
	}
	return fields, nil
}

func requiredResponseField(fields map[string]json.RawMessage, name, path string) (json.RawMessage, error) {
	raw, ok := fields[name]
	if !ok {
		return nil, invalidResponseField(path, "required field is missing")
	}
	return raw, nil
}

func decodeRequiredResponseField(raw json.RawMessage, destination any, path string) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return invalidResponseField(path, "required field is null")
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		return &responseFieldError{path: path, cause: err}
	}
	return nil
}

func responsePointer(base, token string) string {
	token = strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
	return base + "/" + token
}

func decodeSystemOne(data []byte, requestID string) (*SystemOneResponse, error) {
	root, err := responseObject(data, "$")
	if err != nil {
		return nil, err
	}
	modelJSON, err := requiredResponseField(root, "model", "/model")
	if err != nil {
		return nil, err
	}
	var model string
	if err := decodeRequiredResponseField(modelJSON, &model, "/model"); err != nil {
		return nil, err
	}
	if model == "" {
		return nil, invalidResponseField("/model", "must not be empty")
	}

	answersJSON, err := requiredResponseField(root, "answers", "/answers")
	if err != nil {
		return nil, err
	}
	answerFields, err := responseObject(answersJSON, "/answers")
	if err != nil {
		return nil, err
	}
	answerIDs := make([]string, 0, len(answerFields))
	for id := range answerFields {
		answerIDs = append(answerIDs, id)
	}
	sort.Strings(answerIDs)
	answers := make(map[string]Answer, len(answerFields))
	for _, id := range answerIDs {
		answer, err := decodeAnswerAt(answerFields[id], responsePointer("/answers", id))
		if err != nil {
			return nil, err
		}
		answers[id] = answer
	}

	usageJSON, err := requiredResponseField(root, "usage", "/usage")
	if err != nil {
		return nil, err
	}
	usageFields, err := responseObject(usageJSON, "/usage")
	if err != nil {
		return nil, err
	}
	usage, err := decodeUsage(usageFields)
	if err != nil {
		return nil, err
	}
	return &SystemOneResponse{Model: model, Answers: answers, Usage: usage,
		RequestID: requestID, raw: bytes.Clone(data)}, nil
}

func decodeAnswer(raw json.RawMessage) (Answer, error) {
	return decodeAnswerAt(raw, "$")
}

func decodeAnswerAt(raw json.RawMessage, path string) (Answer, error) {
	fields, err := responseObject(raw, path)
	if err != nil {
		return nil, err
	}
	typePath := responsePointer(path, "type")
	typeJSON, err := requiredResponseField(fields, "type", typePath)
	if err != nil {
		return nil, err
	}
	var kind string
	if err := decodeRequiredResponseField(typeJSON, &kind, typePath); err != nil {
		return nil, err
	}
	if kind == "" {
		return nil, invalidResponseField(typePath, "must not be empty")
	}
	owned := bytes.Clone(raw)
	switch kind {
	case "noul":
		noulPath := responsePointer(path, "noul")
		noulJSON, err := requiredResponseField(fields, "noul", noulPath)
		if err != nil {
			return nil, err
		}
		var noul float64
		if err := decodeRequiredResponseField(noulJSON, &noul, noulPath); err != nil {
			return nil, err
		}
		return NoulAnswer{Noul: noul, raw: owned}, nil
	case "choice":
		var answer ChoiceAnswer
		for _, field := range []struct {
			name        string
			destination any
		}{
			{"choice", &answer.Choice},
			{"confidence", &answer.Confidence},
			{"probabilities", &answer.Probabilities},
		} {
			fieldPath := responsePointer(path, field.name)
			fieldJSON, err := requiredResponseField(fields, field.name, fieldPath)
			if err != nil {
				return nil, err
			}
			if err := decodeRequiredResponseField(fieldJSON, field.destination, fieldPath); err != nil {
				return nil, err
			}
		}
		if answer.Probabilities == nil {
			return nil, invalidResponseField(responsePointer(path, "probabilities"), "expected a JSON object")
		}
		answer.raw = owned
		return answer, nil
	case "score":
		var score float64
		var confidence float64
		var wireLegend map[string]Entry
		var wireProbabilities map[string]float64
		for _, field := range []struct {
			name        string
			destination any
		}{
			{"score", &score},
			{"confidence", &confidence},
			{"legend", &wireLegend},
			{"probabilities", &wireProbabilities},
		} {
			fieldPath := responsePointer(path, field.name)
			fieldJSON, err := requiredResponseField(fields, field.name, fieldPath)
			if err != nil {
				return nil, err
			}
			if err := decodeRequiredResponseField(fieldJSON, field.destination, fieldPath); err != nil {
				return nil, err
			}
		}
		legendPath := responsePointer(path, "legend")
		if wireLegend == nil {
			return nil, invalidResponseField(legendPath, "expected a JSON object")
		}
		probabilitiesPath := responsePointer(path, "probabilities")
		if wireProbabilities == nil {
			return nil, invalidResponseField(probabilitiesPath, "expected a JSON object")
		}
		legend := make(map[int]Entry, len(wireLegend))
		probabilities := make(map[int]float64, len(wireProbabilities))
		for key, value := range wireLegend {
			n, err := strconv.Atoi(key)
			if err != nil {
				return nil, invalidResponseField(responsePointer(legendPath, key), "key is not an integer")
			}
			legend[n] = value
		}
		for key, value := range wireProbabilities {
			n, err := strconv.Atoi(key)
			if err != nil {
				return nil, invalidResponseField(responsePointer(probabilitiesPath, key), "key is not an integer")
			}
			probabilities[n] = value
		}
		return ScoreAnswer{Score: score, Confidence: confidence, Legend: legend, Probabilities: probabilities, raw: owned}, nil
	default:
		return UnknownAnswer{Type: kind, raw: owned}, nil
	}
}

func decodeUsage(fields map[string]json.RawMessage) (Usage, error) {
	var usage Usage
	for _, field := range []struct {
		name        string
		destination **int
	}{
		{"input_tokens", &usage.InputTokens},
		{"output_tokens", &usage.OutputTokens},
	} {
		raw, ok := fields[field.name]
		if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			continue
		}
		var value int
		fieldPath := responsePointer("/usage", field.name)
		if err := decodeRequiredResponseField(raw, &value, fieldPath); err != nil {
			return Usage{}, err
		}
		*field.destination = &value
	}
	return usage, nil
}

func decodeModels(data []byte) ([]ModelCard, error) {
	root, err := responseObject(data, "$")
	if err != nil {
		return nil, err
	}
	modelsJSON, err := requiredResponseField(root, "models", "/models")
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(modelsJSON)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, invalidResponseField("/models", "expected a JSON array")
	}
	var rawModels []json.RawMessage
	if err := json.Unmarshal(trimmed, &rawModels); err != nil {
		return nil, &responseFieldError{path: "/models", cause: err}
	}
	models := make([]ModelCard, len(rawModels))
	for i, raw := range rawModels {
		modelPath := responsePointer("/models", strconv.Itoa(i))
		fields, err := responseObject(raw, modelPath)
		if err != nil {
			return nil, err
		}
		for _, field := range []struct {
			name        string
			destination *string
		}{
			{"name", &models[i].Name},
			{"description", &models[i].Description},
			{"release_date", &models[i].ReleaseDate},
		} {
			fieldPath := responsePointer(modelPath, field.name)
			fieldJSON, err := requiredResponseField(fields, field.name, fieldPath)
			if err != nil {
				return nil, err
			}
			if err := decodeRequiredResponseField(fieldJSON, field.destination, fieldPath); err != nil {
				return nil, err
			}
		}
	}
	return models, nil
}

func validateAnswerKinds(answers map[string]Answer, questions Questions) error {
	ids := make([]string, 0, len(questions))
	for id := range questions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		answer, ok := answers[id]
		answerPath := responsePointer("/answers", id)
		if !ok {
			return invalidResponseField(answerPath, "required answer is missing")
		}
		want, err := questionKind(questions[id])
		if err != nil {
			return invalidResponseField(responsePointer(answerPath, "type"), "cannot determine request question type: %v", err)
		}
		got := answerKind(answer)
		if got != want {
			return invalidResponseField(responsePointer(answerPath, "type"), "answer type %q does not match question type %q", got, want)
		}
	}
	return nil
}

func questionKind(question Question) (string, error) {
	switch value := question.(type) {
	case NoulQuestion, *NoulQuestion:
		return "noul", nil
	case ChoiceQuestion, *ChoiceQuestion:
		return "choice", nil
	case ScoreQuestion, *ScoreQuestion:
		return "score", nil
	case RawQuestion:
		return rawQuestionKind(value.JSON)
	case *RawQuestion:
		return rawQuestionKind(value.JSON)
	default:
		return "", fmt.Errorf("unsupported question type %T", question)
	}
}

func rawQuestionKind(raw json.RawMessage) (string, error) {
	fields, err := responseObject(raw, "$")
	if err != nil {
		return "", err
	}
	typeJSON, err := requiredResponseField(fields, "type", "/type")
	if err != nil {
		return "", err
	}
	var kind string
	if err := decodeRequiredResponseField(typeJSON, &kind, "/type"); err != nil {
		return "", err
	}
	return kind, nil
}

func answerKind(answer Answer) string {
	switch value := answer.(type) {
	case NoulAnswer:
		return "noul"
	case ChoiceAnswer:
		return "choice"
	case ScoreAnswer:
		return "score"
	case UnknownAnswer:
		return value.Type
	default:
		return ""
	}
}
