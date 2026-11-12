package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	studentsAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	"github.com/moto-nrw/project-phoenix/roots/owners"
)

type studentNoteReadersFunc func(context.Context, string) ([]*peopleValues.StaffWithRoleInfo, error)

func (f studentNoteReadersFunc) ListStaffWithPermission(ctx context.Context, permission string) ([]*peopleValues.StaffWithRoleInfo, error) {
	return f(ctx, permission)
}

type staffPusherFunc func(context.Context, owners.StaffPush) error

func (f staffPusherFunc) Notify(ctx context.Context, push owners.StaffPush) error {
	return f(ctx, push)
}

// The note readers are the staff holding users:read, the right of the notes
// routes; the push carries the shared-note type and a fresh key for every
// announcement.
func TestStudentNoteTeamReachUsesTheNoteReadRight(t *testing.T) {
	t.Parallel()
	var pushes []owners.StaffPush
	tick := int64(0)
	reach := studentNoteTeamReach{
		staff: studentNoteReadersFunc(func(_ context.Context, permission string) ([]*peopleValues.StaffWithRoleInfo, error) {
			assert.Equal(t, "users:read", permission)
			return []*peopleValues.StaffWithRoleInfo{{AccountID: 4}, nil, {AccountID: 0}, {AccountID: 9}}, nil
		}),
		staffPusher: staffPusherFunc(func(_ context.Context, push owners.StaffPush) error {
			pushes = append(pushes, push)
			return nil
		}),
		now: func() time.Time { tick++; return time.Unix(0, tick) },
	}
	readers, err := reach.NoteReaderAccountIDs(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []int64{4, 9}, readers)

	shared := studentsAPI.StudentNoteSharedPush{StudentID: 3, NoteID: 5, RecipientAccountIDs: readers, DeepLink: "/students/3?tab=notizen"}
	require.NoError(t, reach.NotifyStudentNoteShared(t.Context(), shared))
	require.NoError(t, reach.NotifyStudentNoteShared(t.Context(), shared))
	require.Len(t, pushes, 2)
	assert.Equal(t, deliveryContract.NotificationStaffStudentNoteShared, pushes[0].Type)
	assert.Equal(t, []int64{4, 9}, pushes[0].AccountIDs)
	assert.Equal(t, "/students/3?tab=notizen", pushes[0].DeepLink)
	assert.NotEqual(t, pushes[0].IdempotencyKey, pushes[1].IdempotencyKey, "telling the team again is a new push")
}
