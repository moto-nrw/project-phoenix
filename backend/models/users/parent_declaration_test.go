package users

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeclarationCanonicalContentIgnoresAttachmentOrder(t *testing.T) {
	t.Parallel()
	a := DeclarationAttachmentDigest{AttachmentID: 3, Filename: "a.pdf", SHA256: "aa"}
	b := DeclarationAttachmentDigest{AttachmentID: 7, Filename: "b.pdf", SHA256: "bb"}
	first := &DeclarationVersion{Title: "Ausflug", Body: "Text", Kind: DeclarationKindConsent, Attachments: []DeclarationAttachmentDigest{a, b}}
	second := &DeclarationVersion{Title: "Ausflug", Body: "Text", Kind: DeclarationKindConsent, Attachments: []DeclarationAttachmentDigest{b, a}}
	assert.Equal(t, string(first.CanonicalContent()), string(second.CanonicalContent()))

	changed := &DeclarationVersion{Title: "Ausflug", Body: "Text.", Kind: DeclarationKindConsent, Attachments: []DeclarationAttachmentDigest{a, b}}
	assert.NotEqual(t, string(first.CanonicalContent()), string(changed.CanonicalContent()))
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
	reference := string(base.CanonicalRecord())
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
		assert.NotEqual(t, reference, string(changed.CanonicalRecord()), name)
	}
	// account_id is a nullable foreign key. PostgreSQL clears it when an
	// account is deleted, while the frozen signer data remains as the proof.
	deletedAccount := base
	deletedAccount.AccountID = nil
	assert.Equal(t, reference, string(deletedAccount.CanonicalRecord()))
	// The instant is encoded in UTC, so the zone the row is read back in
	// cannot change the record.
	berlin := base
	berlin.SubmittedAt = base.SubmittedAt.In(time.FixedZone("CEST", 2*60*60))
	assert.Equal(t, reference, string(berlin.CanonicalRecord()))
}
