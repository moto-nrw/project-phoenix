package routetest

import (
	"encoding/json"
	"fmt"
)

// problemTextMembers are the members every API error body carries as
// non-empty strings (ADR 0006, #2507): the RFC 9457 members, the legacy
// `error` text and the code that is the error's identity.
var problemTextMembers = []string{"type", "title", "detail", "error", "code"}

// ProblemEnvelopeViolations lists how an error body departs from the one
// shared error envelope: `status` is always "error", the text travels in
// `error` (never `message`), and type, title, detail, error and code are
// non-empty strings. With requireInstance, `instance` must name the request.
// `errors` and `details` are optional; further members are RFC 9457
// extensions. An empty result means the body has the shared form.
func ProblemEnvelopeViolations(raw []byte, requireInstance bool) []string {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil || body == nil {
		return []string{fmt.Sprintf("not a JSON object: %q", raw)}
	}
	var violations []string
	if status := envelopeString(body, "status"); status != "error" {
		violations = append(violations, fmt.Sprintf(`status is %s, want "error"`, envelopeRaw(body, "status")))
	}
	if _, ok := body["message"]; ok {
		violations = append(violations, `carries "message"; the text belongs in "error"`)
	}
	for _, member := range problemTextMembers {
		if envelopeString(body, member) == "" {
			violations = append(violations, fmt.Sprintf("%s is %s, want a non-empty string", member, envelopeRaw(body, member)))
		}
	}
	if _, ok := body["instance"]; !ok {
		violations = append(violations, "instance is missing")
	} else if requireInstance && envelopeString(body, "instance") == "" {
		violations = append(violations, "instance is empty")
	}
	return violations
}

func envelopeString(body map[string]json.RawMessage, member string) string {
	var value string
	if err := json.Unmarshal(body[member], &value); err != nil {
		return ""
	}
	return value
}

func envelopeRaw(body map[string]json.RawMessage, member string) string {
	value, ok := body[member]
	if !ok {
		return "missing"
	}
	return string(value)
}
