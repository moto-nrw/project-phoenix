package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ProblemResponseMiddleware logs every body it has to repair (#2507). The
// check must flag a handler's own status or a `message` member and leave the
// shared envelope alone.
func TestDepartsFromEnvelope(t *testing.T) {
	t.Parallel()

	for body, want := range map[string]bool{
		`{"status":"error","error":"taken"}`:                      false,
		`{"status":"error","error":"taken","details":{"id":"1"}}`: false,
		`{"status":"Conflict","error":"taken"}`:                   true,
		`{"status":"error","message":"taken"}`:                    true,
		`{"conflicts":[],"message":"taken"}`:                      true,
		`{"status":"success","data":{},"message":"conflict"}`:     true,
	} {
		var decoded map[string]json.RawMessage
		assert.NoError(t, json.Unmarshal([]byte(body), &decoded))
		assert.Equal(t, want, departsFromEnvelope(decoded), body)
	}
}
