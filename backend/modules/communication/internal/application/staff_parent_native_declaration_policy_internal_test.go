package application_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	"github.com/stretchr/testify/assert"
)

func TestParentNativeDeclarationPolicyRecomputesCanonicalCurrentValues(t *testing.T) {
	t.Parallel()
	policy := parentstore.NewPortalDeclarationPolicy()
	version := communication.PortalDeclarationVersion{TenantID: 13, AnnouncementID: 17, Title: "Title", Body: "Original", Kind: "consent", Attachments: []communication.PortalDeclarationAttachmentDigest{{AttachmentID: 29, Filename: "second", SizeBytes: 31}, {AttachmentID: 23, Filename: "first", SizeBytes: 37}}}
	before := policy.PortalCanonicalContent(version)
	assert.Equal(t, []byte(`{"format":1,"title":"Title","body":"Original","kind":"consent","attachments":[{"attachment_id":23,"filename":"first","content_type":"","size_bytes":37,"sha256":""},{"attachment_id":29,"filename":"second","content_type":"","size_bytes":31,"sha256":""}]}`), before)
	version.Body = "Changed after reading"
	assert.NotEqual(t, before, policy.PortalCanonicalContent(version))
	assert.Equal(t, int64(29), version.Attachments[0].AttachmentID, "canonical sorting must not mutate the caller's order")
	guardian := int64(41)
	record := communication.PortalDeclarationSubmission{TenantID: 13, AnnouncementID: 17, VersionID: 19, StudentID: 43, GuardianProfileID: &guardian, SignerName: "Guardian", GuardianRole: "parent", Action: "agreed", Method: "simple_electronic", PasswordConfirmed: true, ContentHash: "content", SubmittedAt: time.Date(2026, 10, 17, 10, 11, 12, 123456000, time.FixedZone("offset", 7200))}
	before = policy.PortalCanonicalRecord(record)
	assert.Equal(t, []byte(`{"format":2,"tenant_id":13,"announcement_id":17,"version_id":19,"content_hash":"content","student_id":43,"guardian_profile_id":41,"signer_name":"Guardian","guardian_role":"parent","action":"agreed","method":"simple_electronic","password_confirmed":true,"submitted_at":"2026-10-17T08:11:12.123456Z"}`), before)
	record.GuardianRole = "changed"
	assert.NotEqual(t, before, policy.PortalCanonicalRecord(record))
}
