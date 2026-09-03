package rooms

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExportSnapshotPreservesFailureClassification(t *testing.T) {
	t.Parallel()
	dataFailure := errors.New("database unavailable")
	renderFailure := errors.New("invalid export options")
	for _, test := range []struct {
		name string
		err  error
		kind FailureKind
		code string
	}{
		{name: "data", err: dataFailure, kind: FailureInternal, code: "internal_error"},
		{name: "renderer", err: &InvalidExportError{Err: renderFailure}, kind: FailureInvalid, code: "invalid_request"},
	} {
		t.Run(test.name, func(t *testing.T) {
			kind, code := classifyExportFailure(test.err)
			require.Equal(t, test.kind, kind)
			require.Equal(t, test.code, code)
		})
	}
}
