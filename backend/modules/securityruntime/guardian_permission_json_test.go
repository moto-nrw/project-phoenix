package securityruntime_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// A stored permissions object grants only a present, decodable, granted
// value; anything that does not decode grants nothing.
func TestGuardianPermissionGrantedJSONFailsClosed(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]bool{
		`not json`:              false,
		`null`:                  false,
		`{}`:                    false,
		`{"other":true}`:        false,
		`{"access":null}`:       false,
		`{"access":false}`:      false,
		`{"access":true}`:       true,
		`{"access":0}`:          true,
		`{"access":"yes"}`:      true,
		`{"access":1e400}`:      true,
		`["access"]`:            false,
		`{"access":true,"x":1}`: true,
	} {
		assert.Equal(t, want, securityruntime.GuardianPermissionGrantedJSON(json.RawMessage(raw), "access"), raw)
	}
}
