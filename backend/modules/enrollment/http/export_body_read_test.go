package enrollmenthttp

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

var errBodyRead = errors.New("connection reset while reading body")

// failingBody fails its first read, as a dropped connection or an exceeded
// body limit does.
type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errBodyRead }

// A body that cannot be read is no empty body: the phase export would
// otherwise drop the child_status filter and export every child.
func TestExportParsersRefuseAnUnreadableBody(t *testing.T) {
	t.Parallel()

	parsers := map[string]func() error{
		"phase export": func() error {
			_, _, err := parsePhaseExportRequest(httptest.NewRequest("POST", "/export", failingBody{}))
			return err
		},
		"care usage export": func() error {
			_, _, err := parseCareUsageExportRequest(httptest.NewRequest("POST", "/export", failingBody{}))
			return err
		},
		"class roster export": func() error {
			_, _, err := parseClassRosterExportRequest(httptest.NewRequest("POST", "/export", failingBody{}))
			return err
		},
	}
	for name, parse := range parsers {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, parse(), errBodyRead)
		})
	}
}
