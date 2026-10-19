package owners

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/require"
)

func TestParentGuardianJSONPermissionsKeepStoredValueSemantics(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		raw  string
		want bool
	}{
		{`null`, false}, {`{}`, false}, {`{"access":null}`, false}, {`{"access":false}`, false}, {`{"access":true}`, true},
		{`{"access":0}`, true}, {`{"access":9007199254740993}`, true}, {`{"access":""}`, true}, {`{"access":[]}`, true}, {`{"access":{}}`, true},
	} {
		t.Run(c.raw, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, c.want, parentGuardianPermissionJSON(json.RawMessage(c.raw), "access"))
		})
	}
}
func TestParentMissingProfileKeepsCanonicalCauseAndMessage(t *testing.T) {
	t.Parallel()
	cause := fmt.Errorf("directory lookup: %w", peopledirectory.ErrGuardianProfileNotFound)
	got := parentGuardianProfileError(cause)
	require.Equal(t, cause.Error(), got.Error())
	require.ErrorIs(t, got, peopledirectory.ErrGuardianProfileNotFound)
	generated := NewParentGuardianPolicy().MissingProfileError()
	require.Equal(t, peopledirectory.ErrGuardianProfileNotFound.Error(), generated.Error())
	require.ErrorIs(t, generated, peopledirectory.ErrGuardianProfileNotFound)
}
