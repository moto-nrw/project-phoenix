package announcement_test

import (
	"context"
	"errors"
	"testing"
	"time"

	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	records "github.com/moto-nrw/project-phoenix/modules/communication/internal/staffannouncements"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pure boundary probes use synthetic snapshot IDs, not database fixtures.
type nativeAnnouncementProbe struct {
	communication.StaffAnnouncementRecords
	create        func(context.Context, *communication.PortalParentAnnouncement) error
	list          func(context.Context, bool) ([]*communication.PortalParentAnnouncement, error)
	insertVersion func(context.Context, *communication.PortalDeclarationVersion) error
	classify      func(error) bool
}

func (p nativeAnnouncementProbe) AnnouncementCreate(ctx context.Context, v *communication.PortalParentAnnouncement) error {
	return p.create(ctx, v)
}
func (p nativeAnnouncementProbe) AnnouncementListForTenant(ctx context.Context, all bool) ([]*communication.PortalParentAnnouncement, error) {
	return p.list(ctx, all)
}
func (p nativeAnnouncementProbe) AnnouncementInsertDeclarationVersion(ctx context.Context, v *communication.PortalDeclarationVersion) error {
	return p.insertVersion(ctx, v)
}
func (p nativeAnnouncementProbe) IsParentAnnouncementPublishedConflict(err error) bool {
	return p.classify(err)
}

func TestNativeAnnouncementWritePreservesCompleteSnapshotsAndPartialErrors(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	at := time.Date(2026, 10, 17, 10, 11, 12, 123456000, time.UTC)
	ref := int64(9007199254740993)
	class, link, system, reminder := "2b", "https://example.test/path", "system", " reminder "
	row := values.StaffParentAnnouncement{ID: 11, CreatedAt: at, UpdatedAt: at.Add(time.Second), TenantID: 13, Title: " title ", Body: " body ", Priority: "important", LinkURL: &link, RequiresAcknowledgement: true, SendEmail: true, PublishedAt: &at, ExpiresAt: &at, Active: true, CreatedBy: 17, ResponseType: "multi_choice", ResponseDeadline: &at, DeliveryMode: "declaration", EmailAudience: "all_contacts", SystemKind: &system, ReminderAt: &at, ReminderText: &reminder, ReminderSentAt: &at, Targets: []*values.StaffParentAnnouncementTarget{{TenantID: 13, ID: 19, AnnouncementID: 11, TargetType: "class", TargetRefID: &ref, TargetRefText: &class, CreatedAt: at}, nil}, Options: []*values.StaffParentAnnouncementOption{{TenantID: 13, ID: 23, AnnouncementID: 11, Label: " option ", Position: 2, CreatedAt: at}, nil}, Declaration: values.StaffAnnouncementDeclarationSettings{Kind: "consent", Signers: "all", Revocable: true, RequiresPassword: true}}
	expected := row
	expected.ID = 29
	expected.TenantID = 31
	expected.CreatedAt = at.Add(time.Minute)
	expected.UpdatedAt = at.Add(2 * time.Minute)
	expected.Body = "normalized before refusal"
	expected.Targets = nil
	expected.Options = []*values.StaffParentAnnouncementOption{}
	cause := errors.New("owner refused after partial normalization")
	calls := 0
	repo := records.NewRepository(nativeAnnouncementProbe{create: func(got context.Context, record *communication.PortalParentAnnouncement) error {
		calls++
		assert.Same(t, ctx, got)
		assert.Equal(t, communication.PortalParentAnnouncement{Poll: true, DeclarationMode: true, ID: 11, CreatedAt: at, UpdatedAt: at.Add(time.Second), TenantID: 13, Title: " title ", Body: " body ", Priority: "important", LinkURL: &link, RequiresAcknowledgement: true, SendEmail: true, PublishedAt: &at, ExpiresAt: &at, Active: true, CreatedBy: 17, ResponseType: "multi_choice", ResponseDeadline: &at, DeliveryMode: "declaration", EmailAudience: "all_contacts", SystemKind: &system, ReminderAt: &at, ReminderText: &reminder, ReminderSentAt: &at, Targets: []*communication.PortalParentAnnouncementTarget{{TenantID: 13, ID: 19, AnnouncementID: 11, TargetType: "class", TargetRefID: &ref, TargetRefText: &class, CreatedAt: at}, nil}, Options: []*communication.PortalParentAnnouncementOption{{TenantID: 13, ID: 23, AnnouncementID: 11, Label: " option ", Position: 2, CreatedAt: at}, nil}, Declaration: communication.PortalAnnouncementDeclarationSettings{Kind: "consent", Signers: "all", Revocable: true, RequiresPassword: true}}, *record)
		record.ID, record.TenantID, record.CreatedAt, record.UpdatedAt, record.Body = expected.ID, expected.TenantID, expected.CreatedAt, expected.UpdatedAt, expected.Body
		record.Targets = nil
		record.Options = []*communication.PortalParentAnnouncementOption{}
		return cause
	}})
	assert.Same(t, cause, repo.Create(ctx, &row))
	assert.Equal(t, 1, calls)
	assert.Equal(t, expected, row)
}

func TestNativeAnnouncementListsPreserveSelectorsPartialResultsAndCollectionPresence(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cause := errors.New("partial read")
	for _, rows := range [][]*communication.PortalParentAnnouncement{nil, {}, {nil, {ID: 9007199254740993, TenantID: 37, Title: "partial", Targets: []*communication.PortalParentAnnouncementTarget{}, Options: nil}}} {
		repo := records.NewRepository(nativeAnnouncementProbe{list: func(got context.Context, all bool) ([]*communication.PortalParentAnnouncement, error) {
			assert.Same(t, ctx, got)
			assert.True(t, all)
			return rows, cause
		}})
		got, err := repo.ListForTenant(ctx, true)
		assert.Same(t, cause, err)
		if rows == nil {
			assert.Nil(t, got)
			continue
		}
		require.NotNil(t, got)
		require.Len(t, got, len(rows))
		if len(rows) > 0 {
			assert.Nil(t, got[0])
			assert.Equal(t, int64(9007199254740993), got[1].ID)
			assert.Equal(t, int64(37), got[1].TenantID)
			assert.Equal(t, "partial", got[1].Title)
			assert.NotNil(t, got[1].Targets)
			assert.Nil(t, got[1].Options)
		}
	}
}

func TestNativeDeclarationWritePreservesCanonicalInputsAndPartialErrors(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 17, 10, 11, 12, 123456000, time.UTC)
	row := values.StaffDeclarationVersion{ID: 41, TenantID: 43, AnnouncementID: 47, VersionNo: 3, Title: "title", Body: "body", Kind: "consent", Attachments: []values.StaffDeclarationAttachmentDigest{{AttachmentID: 53, Filename: "proof.pdf", ContentType: "application/pdf", SizeBytes: 9007199254740993, SHA256: "digest"}}, ContentHash: "content", PublishedAt: at}
	expected := row
	expected.ID = 59
	expected.VersionNo = 5
	expected.ContentHash = "partial hash"
	expected.Attachments = []values.StaffDeclarationAttachmentDigest{}
	cause := errors.New("version conflict")
	repo := records.NewRepository(nativeAnnouncementProbe{insertVersion: func(_ context.Context, v *communication.PortalDeclarationVersion) error {
		assert.Equal(t, communication.PortalDeclarationVersion{ID: 41, TenantID: 43, AnnouncementID: 47, VersionNo: 3, Title: "title", Body: "body", Kind: "consent", Attachments: []communication.PortalDeclarationAttachmentDigest{{AttachmentID: 53, Filename: "proof.pdf", ContentType: "application/pdf", SizeBytes: 9007199254740993, SHA256: "digest"}}, ContentHash: "content", PublishedAt: at}, *v)
		v.ID = expected.ID
		v.VersionNo = expected.VersionNo
		v.ContentHash = expected.ContentHash
		v.Attachments = []communication.PortalDeclarationAttachmentDigest{}
		return cause
	}})
	assert.Same(t, cause, repo.InsertDeclarationVersion(context.Background(), &row))
	assert.Equal(t, expected, row)
}

func TestNativeAnnouncementNilWritesAndConflictClassificationRemainAtSource(t *testing.T) {
	t.Parallel()
	assert.Nil(t, records.NewRepository(nil))
	cause := errors.New("published at source")
	wrapped := errors.Join(errors.New("context"), cause)
	repo := records.NewRepository(nativeAnnouncementProbe{create: func(_ context.Context, v *communication.PortalParentAnnouncement) error {
		assert.Nil(t, v)
		return cause
	}, classify: func(err error) bool { assert.Same(t, wrapped, err); return errors.Is(err, cause) }})
	assert.Same(t, cause, repo.Create(context.Background(), nil))
	assert.True(t, repo.IsPublishedConflict(wrapped))
}
