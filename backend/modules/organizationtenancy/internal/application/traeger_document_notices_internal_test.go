package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// releaseNoticesDouble records the pushes a release enqueues.
type releaseNoticesDouble struct {
	err     error
	notices []organizationtenancy.TraegerDocumentReleaseNotice
}

func (n *releaseNoticesDouble) EnqueueReleaseNotice(_ context.Context, notice organizationtenancy.TraegerDocumentReleaseNotice) error {
	if n.err != nil {
		return n.err
	}
	n.notices = append(n.notices, notice)
	return nil
}

// releaseStoreDouble stores releases with increasing IDs, or refuses them.
type releaseStoreDouble struct {
	documentStoreDouble
	next int64
	err  error
}

func (s *releaseStoreDouble) InsertRelease(_ context.Context, _, _, _ int64, input organizationtenancy.TraegerDocumentReleaseInput, groupName string) (organizationtenancy.TraegerDocumentRelease, error) {
	if s.err != nil {
		return organizationtenancy.TraegerDocumentRelease{}, s.err
	}
	s.next++
	return organizationtenancy.TraegerDocumentRelease{ID: s.next, Audience: input.Audience, SchoolID: input.SchoolID, GroupID: input.GroupID, GroupName: groupName}, nil
}

// TestReleaseForParentsEnqueuesItsPushInTheReleaseTransaction pins #5941: a
// stored release for parents puts one push with its release and document on
// the outbox inside the Träger transaction of the release; a release for
// schools and a refused release put nothing there; a failed enqueue fails the
// release, so the Träger transaction rolls it back.
func TestReleaseForParentsEnqueuesItsPushInTheReleaseTransaction(t *testing.T) {
	t.Parallel()
	notices := &releaseNoticesDouble{}
	store := &releaseStoreDouble{}
	runtime := &downloadRuntimeDouble{}
	service := NewTraegerDocuments(store, organizationtenancy.TraegerDocumentPorts{
		Schools: traegerSchoolsDouble{21}, Groups: groupsDouble{}, Runtime: runtime, Access: &schoolAccessDouble{},
		Downloads: &downloadLogDouble{}, Notices: notices,
	})
	ctx := context.Background()

	release, err := service.Release(ctx, 3, 7, 11, organizationtenancy.TraegerDocumentReleaseInput{Audience: organizationtenancy.TraegerDocumentForParents})
	require.NoError(t, err)
	assert.Equal(t, []organizationtenancy.TraegerDocumentReleaseNotice{{OrganizationID: 3, ReleaseID: release.ID, DocumentID: 11}}, notices.notices)
	assert.Equal(t, []int64{3}, runtime.traeger, "the push is enqueued in the release's one Träger transaction")

	_, err = service.Release(ctx, 3, 7, 11, organizationtenancy.TraegerDocumentReleaseInput{Audience: organizationtenancy.TraegerDocumentForSchools})
	require.NoError(t, err)
	assert.Len(t, notices.notices, 1, "a release for schools tells no parent")

	store.err = organizationtenancy.ErrTraegerDocumentAlreadyReleased
	_, err = service.Release(ctx, 3, 7, 11, organizationtenancy.TraegerDocumentReleaseInput{Audience: organizationtenancy.TraegerDocumentForParents})
	require.ErrorIs(t, err, organizationtenancy.ErrTraegerDocumentAlreadyReleased)
	assert.Len(t, notices.notices, 1, "a repeated release of the same audience pushes nothing")

	store.err = nil
	notices.err = errors.New("outbox unavailable")
	_, err = service.Release(ctx, 3, 7, 11, organizationtenancy.TraegerDocumentReleaseInput{Audience: organizationtenancy.TraegerDocumentForParents})
	require.ErrorIs(t, err, notices.err, "a release whose push cannot be enqueued fails and rolls back")
}
