package messaging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// attachmentCalls records the entry's calls in order. Each audience answer is
// taken from tenants in turn; the last one repeats.
type attachmentCalls struct {
	calls   []string
	tenants []int64
}

func (c *attachmentCalls) tenant() int64 {
	tenantID := c.tenants[0]
	if len(c.tenants) > 1 {
		c.tenants = c.tenants[1:]
	}
	return tenantID
}

func (c *attachmentCalls) GuardianAnnouncementTenant(context.Context, int64, int64) (int64, error) {
	c.calls = append(c.calls, "audience")
	return c.tenant(), nil
}

func (c *attachmentCalls) GuardianDeclarationProofAttachmentTenant(context.Context, int64, int64, int64, int64) (int64, error) {
	c.calls = append(c.calls, "proof")
	return c.tenant(), nil
}

func (c *attachmentCalls) ListAnnouncementAttachmentsInTenant(context.Context, int64, int64) ([]attachmentRow, error) {
	c.calls = append(c.calls, "list")
	return []attachmentRow{{ID: 73}}, nil
}

func (c *attachmentCalls) FindAnnouncementAttachmentInTenant(context.Context, int64, int64, int64) (attachmentRow, error) {
	c.calls = append(c.calls, "find")
	return attachmentRow{ID: 73}, nil
}

// attachmentRow and attachmentContent stand in for File Storage's values.
type (
	attachmentRow     struct{ ID int64 }
	attachmentContent struct{ Filename string }
)

func (c *attachmentCalls) OpenAnnouncementAttachmentInTenant(context.Context, int64, int64, int64) (attachmentContent, error) {
	c.calls = append(c.calls, "open")
	return attachmentContent{Filename: "brief.pdf"}, nil
}

func newAttachmentEntry(t *testing.T, tenants ...int64) (*AnnouncementAttachments[attachmentRow, attachmentContent], *attachmentCalls) {
	t.Helper()
	calls := &attachmentCalls{tenants: tenants}
	entry, err := NewAnnouncementAttachments[attachmentRow, attachmentContent](calls, calls)
	require.NoError(t, err)
	return entry, calls
}

func TestAnnouncementAttachmentsAskTheAudienceAgainBeforeHandingOut(t *testing.T) {
	t.Parallel()
	entry, calls := newAttachmentEntry(t, 17)
	attachments, err := entry.ListGuardianAttachments(t.Context(), 29, 41)
	require.NoError(t, err)
	assert.Equal(t, []attachmentRow{{ID: 73}}, attachments)
	assert.Equal(t, []string{"audience", "list", "audience"}, calls.calls)

	entry, calls = newAttachmentEntry(t, 17)
	content, err := entry.OpenGuardianAttachment(t.Context(), 29, 41, 73)
	require.NoError(t, err)
	assert.Equal(t, "brief.pdf", content.Filename)
	assert.Equal(t, []string{"audience", "find", "audience", "open"}, calls.calls)

	entry, calls = newAttachmentEntry(t, 17)
	_, err = entry.OpenGuardianDeclarationProofAttachment(t.Context(), 29, 41, 53, 73)
	require.NoError(t, err)
	assert.Equal(t, []string{"proof", "find", "proof", "open"}, calls.calls)
}

func TestAnnouncementAttachmentsRefuseOutsideTheAudience(t *testing.T) {
	t.Parallel()
	entry, calls := newAttachmentEntry(t, 0)
	_, err := entry.ListGuardianAttachments(t.Context(), 29, 41)
	require.ErrorIs(t, err, ErrAttachmentNotFound)
	_, err = entry.OpenGuardianAttachment(t.Context(), 29, 41, 73)
	require.ErrorIs(t, err, ErrAttachmentNotFound)
	_, err = entry.OpenGuardianDeclarationProofAttachment(t.Context(), 29, 41, 53, 73)
	require.ErrorIs(t, err, ErrAttachmentNotFound)
	assert.Equal(t, []string{"audience", "audience", "proof"}, calls.calls, "no file is read for a stranger")
}

func TestAnnouncementAttachmentsRefuseWhenTheAudienceChangedMeanwhile(t *testing.T) {
	t.Parallel()
	for name, second := range map[string]int64{"withdrawn": 0, "other school": 19} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry, calls := newAttachmentEntry(t, 17, second)
			_, err := entry.OpenGuardianAttachment(t.Context(), 29, 41, 73)
			require.ErrorIs(t, err, ErrAttachmentNotFound)
			assert.Equal(t, []string{"audience", "find", "audience"}, calls.calls, "the bytes are never opened")

			entry, calls = newAttachmentEntry(t, 17, second)
			_, err = entry.ListGuardianAttachments(t.Context(), 29, 41)
			require.ErrorIs(t, err, ErrAttachmentNotFound)
			assert.Equal(t, []string{"audience", "list", "audience"}, calls.calls)
		})
	}
}

func TestAnnouncementAttachmentsRefuseInvalidIDsWithoutAsking(t *testing.T) {
	t.Parallel()
	entry, calls := newAttachmentEntry(t, 17)
	_, err := entry.ListGuardianAttachments(t.Context(), 0, 41)
	require.ErrorIs(t, err, ErrAttachmentNotFound)
	_, err = entry.OpenGuardianAttachment(t.Context(), 29, 0, 73)
	require.ErrorIs(t, err, ErrAttachmentNotFound)
	_, err = entry.OpenGuardianDeclarationProofAttachment(t.Context(), 29, 41, 0, 73)
	require.ErrorIs(t, err, ErrAttachmentNotFound)
	assert.Empty(t, calls.calls)

	_, err = NewAnnouncementAttachments[attachmentRow, attachmentContent](nil, calls)
	require.Error(t, err)
}
