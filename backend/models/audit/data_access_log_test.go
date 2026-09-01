package audit_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/stretchr/testify/assert"
)

func TestDataAccessLog_Metadata(t *testing.T) {
	t.Parallel()

	d := &audit.DataAccessLog{}
	// GetMetadata lazily initialises the map on a fresh row.
	m := d.GetMetadata()
	assert.NotNil(t, m, "GetMetadata initialises a nil map")
	assert.Empty(t, m)

	// SetMetadata initialises (when still nil) and stores keys.
	d2 := &audit.DataAccessLog{}
	d2.SetMetadata("phase_id", int64(7401))
	d2.SetMetadata("format", "pdf")
	assert.Equal(t, int64(7401), d2.GetMetadata()["phase_id"])
	assert.Equal(t, "pdf", d2.GetMetadata()["format"])
}
