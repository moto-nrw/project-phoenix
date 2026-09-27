package domain

import "time"

// DeclarationSettings are the Erklärung columns of an announcement (#3430).
// Kind is empty for every other delivery mode.
type DeclarationSettings struct {
	Kind             string
	Signers          string
	Revocable        bool
	RequiresPassword bool
}

// DeclarationAttachmentDigest identifies one attachment of a frozen version by
// its content.
type DeclarationAttachmentDigest struct {
	AttachmentID int64
	Filename     string
	ContentType  string
	SizeBytes    int64
	SHA256       string
}

// DeclarationVersion is the frozen wording of one publication.
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

// DeclarationSubmission is one declared action.
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
}

// DeclarationChild is one child a declaration reaches.
type DeclarationChild struct {
	AnnouncementID int64
	StudentID      int64
	FirstName      string
	LastName       string
	SchoolClass    string
}

// DeclarationSigner is one guardian who may declare for a reached child.
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

// DeclarationSignerContext is the relationship a submission is made under.
type DeclarationSignerContext struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
	GuardianRole      string
}
