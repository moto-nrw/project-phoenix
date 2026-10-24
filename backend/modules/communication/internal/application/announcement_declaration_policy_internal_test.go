package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"

	"github.com/stretchr/testify/assert"
)

func TestDeclarationCanonicalContentIgnoresAttachmentOrder(t *testing.T) {
	t.Parallel()
	a := communication.PortalDeclarationAttachmentDigest{AttachmentID: 3, Filename: "a.pdf", SHA256: "aa"}
	b := communication.PortalDeclarationAttachmentDigest{AttachmentID: 7, Filename: "b.pdf", SHA256: "bb"}
	first := &DeclarationVersion{Title: "Ausflug", Body: "Text", Kind: DeclarationKindConsent, Attachments: []communication.PortalDeclarationAttachmentDigest{a, b}}
	second := &DeclarationVersion{Title: "Ausflug", Body: "Text", Kind: DeclarationKindConsent, Attachments: []communication.PortalDeclarationAttachmentDigest{b, a}}
	assert.Equal(t, string(CanonicalDeclarationContent(first)), string(CanonicalDeclarationContent(second)))

	changed := &DeclarationVersion{Title: "Ausflug", Body: "Text.", Kind: DeclarationKindConsent, Attachments: []communication.PortalDeclarationAttachmentDigest{a, b}}
	assert.NotEqual(t, string(CanonicalDeclarationContent(first)), string(CanonicalDeclarationContent(changed)))
}

func TestDeclarationCanonicalRecordCoversEveryImmutableField(t *testing.T) {
	t.Parallel()
	account, profile := int64(24), int64(9)
	base := DeclarationSubmission{
		TenantID: 2, AnnouncementID: 5, VersionID: 8, StudentID: 11, AccountID: &account, GuardianProfileID: &profile,
		SignerName: "Klaus Schneider", GuardianRole: "primary_guardian", Action: DeclarationActionAgreed,
		Method: DeclarationMethodSimpleElectronic, ContentHash: "abc",
		SubmittedAt: time.Date(2026, 9, 27, 8, 30, 0, 123456000, time.UTC),
	}
	reference := string(CanonicalDeclarationRecord(&base))
	mutations := map[string]func(s *DeclarationSubmission){
		"action":   func(s *DeclarationSubmission) { s.Action = DeclarationActionDeclined },
		"student":  func(s *DeclarationSubmission) { s.StudentID = 12 },
		"version":  func(s *DeclarationSubmission) { s.VersionID = 9 },
		"content":  func(s *DeclarationSubmission) { s.ContentHash = "abd" },
		"signer":   func(s *DeclarationSubmission) { s.SignerName = "Anna Schneider" },
		"time":     func(s *DeclarationSubmission) { s.SubmittedAt = s.SubmittedAt.Add(time.Microsecond) },
		"password": func(s *DeclarationSubmission) { s.PasswordConfirmed = true },
	}
	for name, mutate := range mutations {
		changed := base
		mutate(&changed)
		assert.NotEqual(t, reference, string(CanonicalDeclarationRecord(&changed)), name)
	}
	// account_id is a nullable foreign key. PostgreSQL clears it when an
	// account is deleted, while the frozen signer data remains as the proof.
	deletedAccount := base
	deletedAccount.AccountID = nil
	assert.Equal(t, reference, string(CanonicalDeclarationRecord(&deletedAccount)))
	// The instant is encoded in UTC, so the zone the row is read back in
	// cannot change the record.
	berlin := base
	berlin.SubmittedAt = base.SubmittedAt.In(time.FixedZone("CEST", 2*60*60))
	assert.Equal(t, reference, string(CanonicalDeclarationRecord(&berlin)))
}
