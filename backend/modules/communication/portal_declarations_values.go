package communication

import (
	"time"
) // Complete native values retain source metadata, nullable fields and JSON presence.
type PortalAnnouncementDeclarationSettings struct {
	Kind             string
	Signers          string
	Revocable        bool
	RequiresPassword bool
}
type PortalAnnouncementFeedDeclaration struct {
	Kind             string
	Signers          string
	Revocable        bool
	RequiresPassword bool
	Deadline         *time.Time
	Closed           bool
	Version          *PortalDeclarationVersion
	Children         []*PortalAnnouncementFeedDeclarationChild
}
type PortalAnnouncementFeedDeclarationChild struct {
	StudentID      int64
	FirstName      string
	LastName       string
	CanSubmit      bool
	State          string
	MyAction       *string
	MySubmittedAt  *time.Time
	AllowedActions []string
	OtherSigners   []PortalDeclarationSignerState
}
type PortalDeclarationAttachmentDigest struct {
	AttachmentID int64  `json:"attachment_id"`
	Filename     string `json:"filename"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
}
type PortalDeclarationChild struct {
	AnnouncementID int64
	StudentID      int64
	FirstName      string
	LastName       string
	SchoolClass    string
}
type PortalDeclarationSigner struct {
	StudentID         int64
	GuardianProfileID int64
	AccountID         int64
	FirstName         string
	LastName          string
	Email             string
	PortalLocale      string
	GuardianRole      string
}
type PortalDeclarationSignerContext struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
	GuardianRole      string
}
type PortalDeclarationSignerState struct {
	AccountID   int64
	FirstName   string
	LastName    string
	Action      *string
	SubmittedAt *time.Time
}
type PortalDeclarationSubmission struct {
	ID                int64
	TenantID          int64
	AnnouncementID    int64
	VersionID         int64
	VersionNo         int
	StudentID         int64
	AccountID         *int64
	GuardianProfileID *int64
	SignerName        string
	GuardianRole      string
	Action            string
	Method            string
	PasswordConfirmed bool
	ContentHash       string
	RecordHash        string
	SubmittedAt       time.Time
}
type PortalDeclarationVersion struct {
	ID             int64
	TenantID       int64
	AnnouncementID int64
	VersionNo      int
	Title          string
	Body           string
	Kind           string
	Attachments    []PortalDeclarationAttachmentDigest
	ContentHash    string
	PublishedAt    time.Time
}
