package active_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingVisitDeletion struct {
	activeService.StudentPresence
	afterDelete error
	deleted     bool
}

func (p *failingVisitDeletion) DeleteVisit(ctx context.Context, id int64) error {
	if err := p.StudentPresence.DeleteVisit(ctx, id); err != nil {
		return err
	}
	p.deleted = true
	return p.afterDelete
}

func TestVisitDeletionRollsBackAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := testSchoolPresence(t, db)
	presence := &failingVisitDeletion{StudentPresence: module, afterDelete: errors.New("fail after visit delete")}
	svc, broadcaster := newServiceWithBroadcaster(t, db, presence)
	testpkg.SetTenantRuntime(t, svc, db)
	student := testpkg.CreateTestStudent(t, db, "Delete", "Rollback", "3a")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	visit := testpkg.CreateTestVisit(t, db, student.ID, group.ID, time.Now(), nil)
	ctx := testpkg.Ctx(t)

	err := svc.DeleteVisit(ctx, visit.ID)
	require.ErrorIs(t, err, activeService.ErrDatabaseOperation)
	require.True(t, presence.deleted, "fault must occur after the delete")
	stored, err := module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	assert.Equal(t, visit.ID, stored.ID)
	assert.Empty(t, broadcaster.Calls())

	presence.afterDelete = nil
	require.NoError(t, svc.DeleteVisit(ctx, visit.ID))
	_, err = module.FindVisit(ctx, visit.ID)
	require.ErrorIs(t, err, studentpresence.ErrVisitNotFound)
	require.ErrorIs(t, svc.DeleteVisit(ctx, visit.ID), activeService.ErrVisitNotFound, "preserve repeated-delete contract")
}
