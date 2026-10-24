package owners

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
)

type attachmentAudienceStub struct{ tenantID int64 }

func (a attachmentAudienceStub) GuardianAnnouncementTenant(context.Context, int64, int64) (int64, error) {
	return a.tenantID, nil
}

func (a attachmentAudienceStub) GuardianDeclarationProofAttachmentTenant(context.Context, int64, int64, int64, int64) (int64, error) {
	return a.tenantID, nil
}

type attachmentObjectStub struct{ *strings.Reader }

func (attachmentObjectStub) Close() error { return nil }

type attachmentReadsStub struct {
	uploadedAt time.Time
	findErr    error
	tenants    []int64
}

func (r *attachmentReadsStub) ListAnnouncementAttachmentsInTenant(_ context.Context, tenantID, announcementID int64) ([]filestorageModule.Attachment, error) {
	r.tenants = append(r.tenants, tenantID)
	return []filestorageModule.Attachment{{ID: 73, AnnouncementID: announcementID, Filename: "brief.pdf", SizeBytes: 4, ContentType: "application/pdf", UploadedAt: r.uploadedAt}}, nil
}

func (r *attachmentReadsStub) FindAnnouncementAttachmentInTenant(_ context.Context, tenantID, _, _ int64) (filestorageModule.Attachment, error) {
	r.tenants = append(r.tenants, tenantID)
	return filestorageModule.Attachment{ID: 73}, r.findErr
}

func (r *attachmentReadsStub) OpenAnnouncementAttachmentInTenant(_ context.Context, tenantID, _, _ int64) (filestorageModule.Content, error) {
	r.tenants = append(r.tenants, tenantID)
	return filestorageModule.Content{Object: attachmentObjectStub{strings.NewReader("%PDF")}, ModTime: r.uploadedAt, Filename: "brief.pdf", ContentType: "application/pdf"}, nil
}

func TestParentGuardianAttachmentsTranslateTheEntry(t *testing.T) {
	t.Parallel()
	uploadedAt := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	reads := &attachmentReadsStub{uploadedAt: uploadedAt}
	attachments, err := NewParentGuardianAttachments(attachmentAudienceStub{tenantID: 17}, reads)
	require.NoError(t, err)

	list, err := attachments.ListGuardianAttachments(t.Context(), 29, 41)
	require.NoError(t, err)
	assert.Equal(t, []filestorageModule.Attachment{{ID: 73, AnnouncementID: 41, Filename: "brief.pdf", SizeBytes: 4, ContentType: "application/pdf", UploadedAt: uploadedAt}}, list)

	content, err := attachments.OpenGuardianAttachment(t.Context(), 29, 41, 73)
	require.NoError(t, err)
	assert.Equal(t, "brief.pdf", content.Filename)
	assert.Equal(t, uploadedAt, content.ModTime)
	body, err := io.ReadAll(io.NewSectionReader(content.Object, 0, content.Object.Size()))
	require.NoError(t, err)
	assert.Equal(t, "%PDF", string(body))
	require.NoError(t, content.Close())
	assert.Equal(t, []int64{17, 17, 17}, reads.tenants, "every read runs under the audience's school")
}

func TestParentGuardianAttachmentsAnswerRefusalsWithFileStorageNotFound(t *testing.T) {
	t.Parallel()
	attachments, err := NewParentGuardianAttachments(attachmentAudienceStub{}, &attachmentReadsStub{})
	require.NoError(t, err)
	_, err = attachments.ListGuardianAttachments(t.Context(), 29, 41)
	require.ErrorIs(t, err, filestorageModule.ErrAttachmentNotFound)
	_, err = attachments.OpenGuardianDeclarationProofAttachment(t.Context(), 29, 41, 53, 73)
	require.ErrorIs(t, err, filestorageModule.ErrAttachmentNotFound)

	broken := errors.New("storage unavailable")
	attachments, err = NewParentGuardianAttachments(attachmentAudienceStub{tenantID: 17}, &attachmentReadsStub{findErr: broken})
	require.NoError(t, err)
	_, err = attachments.OpenGuardianAttachment(t.Context(), 29, 41, 73)
	require.ErrorIs(t, err, broken, "a storage failure stays a failure")

	_, err = NewParentGuardianAttachments(attachmentAudienceStub{}, nil)
	require.Error(t, err)
}
