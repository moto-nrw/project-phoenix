package messaging

import (
	"time"
) // Complete native values retain source metadata, nullable fields and JSON presence.
type AnnouncementDeclarationSettings struct {
	Kind              string
	Signers           string
	Revocable         bool
	RequiresPassword  bool
	RequiresSignature bool
}
type AnnouncementFeedDeclaration struct {
	Kind              string
	Signers           string
	Revocable         bool
	RequiresPassword  bool
	RequiresSignature bool
	Deadline          *time.Time
	Closed            bool
	Version           *DeclarationVersion
	Children          []*AnnouncementFeedDeclarationChild
}
type AnnouncementFeedDeclarationChild struct {
	StudentID      int64
	FirstName      string
	LastName       string
	CanSubmit      bool
	State          string
	MyAction       *string
	MySubmittedAt  *time.Time
	AllowedActions []string
	OtherSigners   []DeclarationSignerState
}
type DeclarationAttachmentDigest struct {
	AttachmentID int64  `json:"attachment_id"`
	Filename     string `json:"filename"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
}
type DeclarationChild struct {
	AnnouncementID int64
	StudentID      int64
	FirstName      string
	LastName       string
	SchoolClass    string
}
type DeclarationSigner struct {
	StudentID         int64
	GuardianProfileID int64
	AccountID         int64
	FirstName         string
	LastName          string
	Email             string
	PortalLocale      string
	GuardianRole      string
}
type DeclarationSignerContext struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
	GuardianRole      string
}
type DeclarationSignerState struct {
	AccountID   int64
	FirstName   string
	LastName    string
	Action      *string
	SubmittedAt *time.Time
}
type DeclarationSubmission struct {
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
	SignatureID       *int64
}
type DeclarationVersion struct {
	ID             int64
	TenantID       int64
	AnnouncementID int64
	VersionNo      int
	Title          string
	Body           string
	Kind           string
	Attachments    []DeclarationAttachmentDigest
	ContentHash    string
	PublishedAt    time.Time
}
