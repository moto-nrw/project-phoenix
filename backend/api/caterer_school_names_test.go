package api

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type catererSchoolDirectory struct {
	organizationtenancy.Capability
	schools []organizationtenancy.School
}

func (d catererSchoolDirectory) ListSchoolsByID(context.Context, []int64) ([]organizationtenancy.School, error) {
	return d.schools, nil
}

func TestCatererSchoolNamesExcludeDeletedSchools(t *testing.T) {
	t.Parallel()
	deletedAt := time.Now()
	directory := catererSchoolDirectory{schools: []organizationtenancy.School{
		{ID: 11, Name: "Schule"},
		{ID: 12, Name: "Gelöschte Schule", DeletedAt: &deletedAt},
	}}
	names, err := schoolNames(directory)(context.Background(), []int64{11, 12})
	require.NoError(t, err)
	assert.Equal(t, map[int64]string{11: "Schule"}, names)
}
