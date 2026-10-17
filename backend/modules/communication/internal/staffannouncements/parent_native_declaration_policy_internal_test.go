package announcement

import (
	"testing"
	"time"

	models "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	"github.com/stretchr/testify/assert"
)

func TestParentNativeDeclarationPolicyRecomputesCanonicalCurrentValues(t *testing.T) {
	t.Parallel()
	policy := parentstore.NewPortalDeclarationPolicy()
	version := communication.PortalDeclarationVersion{TenantID: 13, AnnouncementID: 17, Title: "Title", Body: "Original", Kind: "consent", Attachments: []communication.PortalDeclarationAttachmentDigest{{AttachmentID: 29, Filename: "second", SizeBytes: 31}, {AttachmentID: 23, Filename: "first", SizeBytes: 37}}}
	original := models.DeclarationVersion{Title: version.Title, Body: version.Body, Kind: version.Kind, Attachments: []models.DeclarationAttachmentDigest{{AttachmentID: 29, Filename: "second", SizeBytes: 31}, {AttachmentID: 23, Filename: "first", SizeBytes: 37}}}
	before := policy.PortalCanonicalContent(version)
	assert.Equal(t, original.CanonicalContent(), before)
	version.Body = "Changed after reading"
	assert.NotEqual(t, before, policy.PortalCanonicalContent(version))
	assert.Equal(t, int64(29), version.Attachments[0].AttachmentID, "canonical sorting must not mutate the caller's order")
	guardian := int64(41)
	record := communication.PortalDeclarationSubmission{TenantID: 13, AnnouncementID: 17, VersionID: 19, StudentID: 43, GuardianProfileID: &guardian, SignerName: "Guardian", GuardianRole: "parent", Action: "agreed", Method: "simple_electronic", PasswordConfirmed: true, ContentHash: "content", SubmittedAt: time.Date(2026, 10, 17, 10, 11, 12, 123456000, time.FixedZone("offset", 7200))}
	legacy := models.DeclarationSubmission{TenantID: record.TenantID, AnnouncementID: record.AnnouncementID, VersionID: record.VersionID, StudentID: record.StudentID, GuardianProfileID: record.GuardianProfileID, SignerName: record.SignerName, GuardianRole: record.GuardianRole, Action: record.Action, Method: record.Method, PasswordConfirmed: record.PasswordConfirmed, ContentHash: record.ContentHash, SubmittedAt: record.SubmittedAt}
	before = policy.PortalCanonicalRecord(record)
	assert.Equal(t, legacy.CanonicalRecord(), before)
	record.GuardianRole = "changed"
	assert.NotEqual(t, before, policy.PortalCanonicalRecord(record))
}
