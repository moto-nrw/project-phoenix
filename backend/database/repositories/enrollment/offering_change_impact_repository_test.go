package enrollment_test

import (
	"testing"

	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOfferingChangeImpactRepositoryKeepsEveryLegacyOfferingLink(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Shared legacy course")
	repository := enrollmentRepo.NewOfferingChangeImpactRepository(db)

	groups, err := repository.CourseGroupsForOfferings(ctx, []enrollmentModels.CourseOfferingReference{
		{OfferingID: 101, ActivityGroupID: &group.ID},
		{OfferingID: 102, ActivityGroupID: &group.ID},
	})

	require.NoError(t, err)
	want := []enrollmentModels.CourseGroup{{
		ID: group.ID, Active: true, ParticipantLimit: group.ParticipantLimit(),
	}}
	assert.Equal(t, want, groups[101])
	assert.Equal(t, want, groups[102])
}
