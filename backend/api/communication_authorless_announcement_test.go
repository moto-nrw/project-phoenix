package api

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An announcement keeps its place when its author's operator account is
// deleted (#4047): it stays editable, and its author stays empty.
func TestAnnouncementWithoutAuthorStaysEditable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildCommunication(t, db, &auditRecorder{})
	ctx := testpkg.Ctx(t)
	author := testpkg.CreateTestOperator(t, db)
	editor := testpkg.CreateTestOperator(t, db)
	announcement := newAnnouncement("Ohne Autor", testpkg.Tenant(t))
	require.NoError(t, module.CreateAnnouncement(ctx, announcement, author.ID, nil))
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM platform.announcements WHERE id = ?`, announcement.ID)
		require.NoError(t, err)
	})
	_, err := db.ExecContext(context.Background(), `UPDATE platform.announcements SET created_by = NULL WHERE id = ?`, announcement.ID)
	require.NoError(t, err)

	stored, err := module.GetAnnouncement(ctx, announcement.ID)
	require.NoError(t, err)
	assert.Zero(t, stored.CreatedBy)
	announcements, err := module.ListAnnouncements(ctx, true)
	require.NoError(t, err)
	found := false
	for _, item := range announcements {
		if item.ID == announcement.ID {
			found = true
			assert.Zero(t, item.CreatedBy)
			break
		}
	}
	assert.True(t, found)
	stored.Title = "Ohne Autor, bearbeitet"
	require.NoError(t, module.UpdateAnnouncement(ctx, stored, editor.ID, nil))

	var authorless bool
	require.NoError(t, db.NewRaw(`SELECT created_by IS NULL FROM platform.announcements WHERE id = ? AND title = ?`,
		announcement.ID, "Ohne Autor, bearbeitet").Scan(context.Background(), &authorless))
	assert.True(t, authorless, "the edit keeps the author empty instead of writing 0")

	err = module.CreateAnnouncement(ctx, newAnnouncement("Ohne Operator", testpkg.Tenant(t)), 0, nil)
	assert.ErrorIs(t, err, communication.ErrInvalidAnnouncement, "a new announcement names its operator")
}

func TestAnnouncementWithoutAuthorReachesItsAudience(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildCommunication(t, db, &auditRecorder{})
	ctx := testpkg.Ctx(t)
	author := testpkg.CreateTestOperator(t, db)
	ownAnnouncementsByOperator(t, db, author.ID)
	_, reader := testpkg.CreateTestPersonWithAccount(t, db, "Ada", "Leserin")
	tenantID := testpkg.Tenant(t)
	announcement := newAnnouncement("Ohne Autor sichtbar", tenantID)
	require.NoError(t, module.CreateAnnouncement(ctx, announcement, author.ID, nil))
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM platform.announcements WHERE id = ?`, announcement.ID)
		require.NoError(t, err)
	})
	require.NoError(t, module.PublishAnnouncement(ctx, announcement.ID, author.ID, nil))
	_, err := db.ExecContext(context.Background(), `UPDATE platform.announcements SET created_by = NULL WHERE id = ?`, announcement.ID)
	require.NoError(t, err)

	announcements, err := module.GetUnreadForUser(ctx, reader.ID, nil, tenantID, tenantOrganizationID(t, db, tenantID))
	require.NoError(t, err)
	require.Len(t, announcements, 1)
	assert.Equal(t, announcement.ID, announcements[0].ID)
	assert.Zero(t, announcements[0].CreatedBy)
}
