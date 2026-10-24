package routetest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// problemTextMembers are the members every API error body carries as
// non-empty strings (ADR 0006, #2520): the RFC 9457 members and the code that
// is the error's identity.
var problemTextMembers = []string{"type", "title", "detail", "code"}

// ProblemEnvelopeViolations lists how an error body answered with status
// departs from the one shared error envelope (RFC 9457): `status` is the
// HTTP status number, the text travels in `detail` (never `error` or
// `message`), type, title, detail and code are non-empty strings, and the
// code is registered in error-registry.json. With requireInstance,
// `instance` must name the request. `errors` and `details` are optional;
// further members are RFC 9457 extensions. An empty result means the body
// has the shared form.
func ProblemEnvelopeViolations(raw []byte, status int, requireInstance bool) []string {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil || body == nil {
		return []string{fmt.Sprintf("not a JSON object: %q", raw)}
	}
	violations := legacyMemberViolations(body, status)
	for _, member := range problemTextMembers {
		if envelopeString(body, member) == "" {
			violations = append(violations, fmt.Sprintf("%s is %s, want a non-empty string", member, envelopeRaw(body, member)))
		}
	}
	if code := envelopeString(body, "code"); code != "" && !IsRegisteredCode(code) {
		violations = append(violations, fmt.Sprintf("code %q is not in error-registry.json", code))
	}
	if _, ok := body["instance"]; !ok {
		violations = append(violations, "instance is missing")
	} else if requireInstance && envelopeString(body, "instance") == "" {
		violations = append(violations, "instance is empty")
	}
	return violations
}

// legacyMemberViolations names the members a body carries from before the
// RFC 9457 envelope: a status other than the HTTP status number and the
// `error` or `message` text.
func legacyMemberViolations(body map[string]json.RawMessage, status int) []string {
	var violations []string
	var number int
	if err := json.Unmarshal(body["status"], &number); err != nil || number != status {
		violations = append(violations, fmt.Sprintf("status is %s, want %d", envelopeRaw(body, "status"), status))
	}
	for _, member := range []string{"error", "message"} {
		if _, ok := body[member]; ok {
			violations = append(violations, fmt.Sprintf(`carries %q; the text belongs in "detail"`, member))
		}
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

var registeredCodes = sync.OnceValue(func() map[string]bool {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("routetest: locate error-registry.json")
	}
	encoded, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "error-registry.json"))
	if err != nil {
		panic("routetest: read error-registry.json: " + err.Error())
	}
	var registry struct {
		Codes []struct {
			Code string `json:"code"`
		} `json:"codes"`
	}
	if err := json.Unmarshal(encoded, &registry); err != nil {
		panic("routetest: decode error-registry.json: " + err.Error())
	}
	codes := make(map[string]bool, len(registry.Codes))
	for _, entry := range registry.Codes {
		codes[entry.Code] = true
	}
	return codes
})

// IsRegisteredCode reports whether code is in error-registry.json. Test
// support may not import api/common, so it reads the registry itself.
func IsRegisteredCode(code string) bool {
	return registeredCodes()[code]
}

// Registered class codes (error-registry.json). Test support may not import
// api/common, so it names them once here.
const (
	codeGeneralInput             = "general.input"
	codeGeneralPermission        = "general.permission"
	codeGeneralRequestTooLarge   = "general.request_too_large"
	codeGeneralBusinessRejection = "general.business_rejection"
	codeGeneralUnavailable       = "general.unavailable"
	codeGeneralServer            = "general.server"
)

// ClassCode is the class code api/common.ErrorClassCode answers a status
// without a code of its own with; the stand-in renderers use it.
// TestRoutetestClassCodesMatchTheSharedOnes keeps the two in step.
func ClassCode(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return codeGeneralPermission
	case http.StatusRequestEntityTooLarge, http.StatusRequestHeaderFieldsTooLarge:
		return codeGeneralRequestTooLarge
	case http.StatusConflict, http.StatusGone, http.StatusUnprocessableEntity:
		return codeGeneralBusinessRejection
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, 499:
		return codeGeneralUnavailable
	default:
		if status >= http.StatusInternalServerError {
			return codeGeneralServer
		}
		return codeGeneralInput
	}
}
