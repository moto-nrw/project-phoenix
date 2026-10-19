package compose_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memoryObjects keeps the objects of one test in memory.
type memoryObjects struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (m *memoryObjects) key(kind string, tenantID int64, storedName string) string {
	return fmt.Sprintf("%s/%d/%s", kind, tenantID, storedName)
}

func (m *memoryObjects) SavePrivate(_ context.Context, kind string, tenantID int64, storedName string, source io.Reader) (int64, error) {
	content, err := io.ReadAll(source)
	if err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[m.key(kind, tenantID, storedName)] = content
	return int64(len(content)), nil
}

func (m *memoryObjects) OpenPrivate(_ context.Context, kind string, tenantID int64, storedName string) (interface {
	io.ReadSeekCloser
	ModTime() time.Time
}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	content, found := m.objects[m.key(kind, tenantID, storedName)]
	if !found {
		return nil, fmt.Errorf("object %s not found", storedName)
	}
	return memoryObject{Reader: bytes.NewReader(content)}, nil
}

func (m *memoryObjects) RemovePrivate(_ context.Context, kind string, tenantID int64, storedName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, m.key(kind, tenantID, storedName))
	return nil
}

type memoryObject struct{ *bytes.Reader }

func (memoryObject) Close() error       { return nil }
func (memoryObject) ModTime() time.Time { return time.Time{} }

func TestNewAttachmentPurgerRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	objects := &memoryObjects{objects: map[string][]byte{}}
	observe := func(compose.Observation) {}

	for name, dependencies := range map[string]compose.AttachmentPurgerDependencies{
		"database": {Objects: objects, Observe: observe},
		"objects":  {DB: db, Observe: observe},
		"observer": {DB: db, Objects: objects},
	} {
		_, err := compose.NewAttachmentPurger(dependencies)
		require.EqualError(t, err, "file storage compose: all dependencies are required", name)
	}
}

// The purger answers the announcement service from the attachment rows and
// objects alone: no announcement service and no parents portal exist yet.
func TestAttachmentPurgerAnswersFromTheAttachmentsAlone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	_, author := testpkg.CreateTestPersonWithAccount(t, db, "Leitung", fmt.Sprintf("Purger-%d", time.Now().UnixNano()))
	announcement := testpkg.CreateTestParentAnnouncement(t, db, author.ID, "Ausflug")
	objects := &memoryObjects{objects: map[string][]byte{}}
	storedName := fmt.Sprintf("purger-%d.pdf", time.Now().UnixNano())
	_, err := objects.SavePrivate(ctx, "announcement-attachments", testpkg.Tenant(t), storedName, bytes.NewReader([]byte("%PDF-")))
	require.NoError(t, err)
	var attachmentID int64
	require.NoError(t, db.NewRaw(`INSERT INTO documents.announcement_attachments
		(tenant_id, announcement_id, filename_display, filename_stored, size_bytes, content_type, uploaded_by)
		VALUES (?, ?, 'Elternbrief.pdf', ?, 5, 'application/pdf', ?) RETURNING id`,
		testpkg.Tenant(t), announcement.ID, storedName, author.ID).Scan(ctx, &attachmentID))

	var operations []string
	purger, err := compose.NewAttachmentPurger(compose.AttachmentPurgerDependencies{DB: db, Objects: objects, Observe: func(observation compose.Observation) {
		operations = append(operations, observation.Operation)
	}})
	require.NoError(t, err)

	count, err := purger.CountAttachments(ctx, announcement.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	digests, err := purger.AttachmentDigests(ctx, announcement.ID, func(source io.Reader) (string, int64, error) {
		content, err := io.ReadAll(source)
		return string(content), int64(len(content)), err
	})
	require.NoError(t, err)
	assert.Equal(t, []filestorage.AttachmentDigest{{
		AttachmentID: attachmentID, Filename: "Elternbrief.pdf", ContentType: "application/pdf", SizeBytes: 5, SHA256: "%PDF-",
	}}, digests)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, purger.QueueAttachmentCleanupForAnnouncement, announcement.ID))
	queued, err := db.NewSelect().TableExpr("documents.announcement_attachment_cleanup").
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("owner_id = ?", announcement.ID).Where("filename_stored = ?", storedName).Count(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, queued, "the cleanup intent precedes the deletion")
	_, err = purger.CountAttachments(ctx, 0)
	require.ErrorIs(t, err, filestorage.ErrInvalid, "the purger answers with the public errors")
	assert.Equal(t, []string{"count_attachments", "attachment_digests", "queue_attachment_cleanup_for_announcement", "count_attachments"}, operations)
}
