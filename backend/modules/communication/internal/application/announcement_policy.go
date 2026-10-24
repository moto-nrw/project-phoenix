package application

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// Announcement priority levels (mirrors the chk_parent_announcements_priority
// constraint). "important" only changes presentation (a highlighted badge); it
// has no effect on targeting or delivery.
const (
	ParentAnnouncementPriorityInfo      = "info"
	ParentAnnouncementPriorityImportant = "important"
)
const (
	ParentAnnouncementResponseNone         = "none"
	ParentAnnouncementResponseSingleChoice = "single_choice"
	ParentAnnouncementResponseMultiChoice  = "multi_choice"
)
const (
	ParentAnnouncementDeliveryStandard = "standard"
	ParentAnnouncementDeliveryLetter   = "letter"
)
const (
	EmailAudiencePortalOnly  = "portal_only"
	EmailAudienceAllContacts = "all_contacts"
)
const (
	AnnouncementTargetSchoolAll         = "school_all"
	AnnouncementTargetClass             = "class"
	AnnouncementTargetGroup             = "group"
	AnnouncementTargetActivityGroup     = "activity_group"
	AnnouncementTargetStudent           = "student"
	AnnouncementTargetPendingEnrollment = "pending_enrollment"
)
const (
	ParentAnnouncementSystemKindCareCancellation = "care_cancellation"
)
const ParentAnnouncementDeliveryDeclaration = "declaration"
const DeclarationKindConsent = "consent"
const (
	DeclarationSignersAny = "any"
	DeclarationSignersAll = "all"
)
const (
	DeclarationActionAgreed   = "agreed"
	DeclarationActionDeclined = "declined"
	DeclarationActionRevoked  = "revoked"
)
const DeclarationMethodSimpleElectronic = "simple_electronic"
const (
	DeclarationStateOpen     = "open"
	DeclarationStatePartial  = "partial"
	DeclarationStateAgreed   = "agreed"
	DeclarationStateDeclined = "declined"
	DeclarationStateRevoked  = "revoked"
	DeclarationStateNoSigner = "no_signer"
	DeclarationStateExpired  = "expired"
)

// ValidAnnouncementResponseType reports whether t is a known response type.
func ValidAnnouncementResponseType(t string) bool {
	switch t {
	case ParentAnnouncementResponseNone,
		ParentAnnouncementResponseSingleChoice,
		ParentAnnouncementResponseMultiChoice:
		return true
	default:
		return false
	}
}

// ValidAnnouncementDeliveryMode reports whether m is a known delivery mode.
func ValidAnnouncementDeliveryMode(m string) bool {
	return m == ParentAnnouncementDeliveryStandard || m == ParentAnnouncementDeliveryLetter ||
		m == ParentAnnouncementDeliveryDeclaration
}

// ValidAnnouncementEmailAudience reports whether a is a known e-mail audience.
func ValidAnnouncementEmailAudience(a string) bool {
	return a == EmailAudiencePortalOnly || a == EmailAudienceAllContacts
}

// ValidAnnouncementPriority reports whether p is a known priority.
func ValidAnnouncementPriority(p string) bool {
	return p == ParentAnnouncementPriorityInfo || p == ParentAnnouncementPriorityImportant
}

// ValidAnnouncementTargetType reports whether t is a known target type.
func ValidAnnouncementTargetType(t string) bool {
	switch t {
	case AnnouncementTargetSchoolAll, AnnouncementTargetClass, AnnouncementTargetGroup,
		AnnouncementTargetActivityGroup, AnnouncementTargetStudent, AnnouncementTargetPendingEnrollment:
		return true
	default:
		return false
	}
}

// IsPublished reports whether the announcement has been published (vs draft).
func AnnouncementIsPublished(a *ParentAnnouncement) bool {
	return a.PublishedAt != nil
}

// IsLetter reports whether the announcement is an Elternbrief — the mode in
// which both channels are mandatory and the mail carries the full body.
func AnnouncementIsLetter(a *ParentAnnouncement) bool {
	return a.DeliveryMode == ParentAnnouncementDeliveryLetter
}

// ReachesContactsWithoutPortal reports whether the e-mail goes beyond the portal
// audience to guardians that have an address but no portal access.
func AnnouncementReachesContactsWithoutPortal(a *ParentAnnouncement) bool {
	return a.EmailAudience == EmailAudienceAllContacts
}

// IsSystem reports whether the system authored the announcement.
func AnnouncementIsSystem(a *ParentAnnouncement) bool {
	return a.SystemKind != nil && *a.SystemKind != ""
}

// IsPoll reports whether the announcement asks guardians a question.
func AnnouncementIsPoll(a *ParentAnnouncement) bool {
	return AnnouncementResponseIsPoll(a.ResponseType)
}

// HasReminder reports whether a scheduled reminder is set (sent or not).
func AnnouncementHasReminder(a *ParentAnnouncement) bool {
	return a.ReminderAt != nil
}

// ReminderSent reports whether the scheduled reminder already went out.
func AnnouncementReminderSent(a *ParentAnnouncement) bool {
	return a.ReminderSentAt != nil
}

// ReminderBody is the wording the reminder carries: the school's own short
// text when it wrote one, otherwise the announcement text.
func AnnouncementReminderBody(a *ParentAnnouncement) string {
	if a.ReminderText != nil && *a.ReminderText != "" {
		return *a.ReminderText
	}
	return a.Body
}

// AcceptsResponsesAt reports whether the poll still accepts answers at t: it
// must be a poll and either have no deadline or one that has not passed.
// Visibility (active/published/expires_at) is checked separately — a closed poll
// stays readable.
func AnnouncementAcceptsResponsesAt(a *ParentAnnouncement, t time.Time) bool {
	if !AnnouncementIsPoll(a) {
		return false
	}
	return a.ResponseDeadline == nil || a.ResponseDeadline.After(t)
}

// IsDeclaration reports whether the announcement is an Erklärung.
func AnnouncementIsDeclaration(a *ParentAnnouncement) bool {
	return a.DeliveryMode == ParentAnnouncementDeliveryDeclaration
}

// The format values version the canonical encodings below. A change to one
// encoding must bump only its format, never reinterpret stored hashes.
const (
	declarationContentHashFormat = 1
	declarationRecordHashFormat  = 2
)

type canonicalDeclarationVersion struct {
	Format      int                                               `json:"format"`
	Title       string                                            `json:"title"`
	Body        string                                            `json:"body"`
	Kind        string                                            `json:"kind"`
	Attachments []communication.PortalDeclarationAttachmentDigest `json:"attachments"`
}

// CanonicalContent is the canonical JSON of what the guardian is shown:
// title, body, kind and every attachment's name, type, size and content
// digest, in attachment-id order. Its SHA-256 is the version's content hash;
// two versions with the same hash showed the same content.
// CanonicalContent is the canonical JSON of what the guardian is shown:
// title, body, kind and every attachment's name, type, size and content
// digest, in attachment-id order. Its SHA-256 is the version's content hash;
// two versions with the same hash showed the same content.
func CanonicalDeclarationContent(v *DeclarationVersion) []byte {
	attachments := append([]communication.PortalDeclarationAttachmentDigest(nil), v.Attachments...)
	sort.Slice(attachments, func(i, j int) bool { return attachments[i].AttachmentID < attachments[j].AttachmentID })
	if attachments == nil {
		attachments = []communication.PortalDeclarationAttachmentDigest{}
	}
	return canonicalJSON(canonicalDeclarationVersion{
		Format: declarationContentHashFormat, Title: v.Title, Body: v.Body, Kind: v.Kind, Attachments: attachments,
	})
}

type canonicalDeclarationSubmission struct {
	Format            int    `json:"format"`
	TenantID          int64  `json:"tenant_id"`
	AnnouncementID    int64  `json:"announcement_id"`
	VersionID         int64  `json:"version_id"`
	ContentHash       string `json:"content_hash"`
	StudentID         int64  `json:"student_id"`
	GuardianProfileID *int64 `json:"guardian_profile_id"`
	SignerName        string `json:"signer_name"`
	GuardianRole      string `json:"guardian_role"`
	Action            string `json:"action"`
	Method            string `json:"method"`
	PasswordConfirmed bool   `json:"password_confirmed"`
	SubmittedAt       string `json:"submitted_at"`
	// SignatureID binds a signed answer to its signature (#4785). It is left
	// out when empty, so every unsigned record keeps the hash it was stored
	// with.
	SignatureID *int64 `json:"signature_id,omitempty"`
}

// CanonicalRecord is the canonical JSON of the submission, including the
// content hash of the version it refers to. Its SHA-256 is the record hash,
// which makes a later change to a stored row detectable; it proves neither
// the signer's identity nor a trusted time on its own. AccountID is excluded:
// account deletion clears that nullable foreign key, while the frozen signer
// identity remains in this record.
// CanonicalRecord is the canonical JSON of the submission, including the
// content hash of the version it refers to. Its SHA-256 is the record hash,
// which makes a later change to a stored row detectable; it proves neither
// the signer's identity nor a trusted time on its own. AccountID is excluded:
// account deletion clears that nullable foreign key, while the frozen signer
// identity remains in this record.
func CanonicalDeclarationRecord(s *DeclarationSubmission) []byte {
	return canonicalJSON(canonicalDeclarationSubmission{
		Format: declarationRecordHashFormat, TenantID: s.TenantID, AnnouncementID: s.AnnouncementID,
		VersionID: s.VersionID, ContentHash: s.ContentHash, StudentID: s.StudentID,
		GuardianProfileID: s.GuardianProfileID, SignerName: s.SignerName, GuardianRole: s.GuardianRole,
		Action: s.Action, Method: s.Method, PasswordConfirmed: s.PasswordConfirmed,
		SubmittedAt: s.SubmittedAt.UTC().Format(time.RFC3339Nano), SignatureID: s.SignatureID,
	})
}

func canonicalJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		// The canonical structs hold only strings, numbers and booleans.
		panic(fmt.Sprintf("users: encode declaration canonical form: %v", err))
	}
	return encoded
}

// Fulfilled reports whether someone has confirmed the letter for this child.
func AnnouncementLetterFulfilled(c *AnnouncementLetterChildStatus) bool {
	return c.AcknowledgedAt != nil
}

// Outstanding reports whether this child is still waiting for a confirmation
// that could actually arrive. A child nobody can confirm for is NOT outstanding
// — it is a data gap, and lumping the two together is what made a school-wide
// letter look almost complete when nearly nobody could confirm.
func AnnouncementLetterOutstanding(c *AnnouncementLetterChildStatus) bool {
	return c.CanConfirm && !AnnouncementLetterFulfilled(c)
}

func AnnouncementResponseIsPoll(responseType string) bool {
	return responseType == ParentAnnouncementResponseSingleChoice || responseType == ParentAnnouncementResponseMultiChoice
}
