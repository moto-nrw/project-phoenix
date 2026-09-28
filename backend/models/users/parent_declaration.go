package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// Erklärungen (#3430). An Erklärung is the third delivery mode of a parent
// announcement: guardians give or refuse their consent for a child (the
// Einverständnis) and every answer is kept as proof. The announcement
// columns carry the settings; the versions freeze what was declared; the
// submissions record who declared what, when.

// ParentAnnouncementDeliveryDeclaration is the delivery mode of an Erklärung
// (mirrors chk_parent_announcements_delivery_mode).
const ParentAnnouncementDeliveryDeclaration = "declaration"

// DeclarationKindConsent is the only declaration kind (mirrors
// chk_parent_announcements_declaration_shape): Zustimmung or Ablehnung. A
// plain read confirmation is the Elternbrief's Lesebestätigung. The kind is
// still stored so every proof names what was asked.
const DeclarationKindConsent = "consent"

// Declaration signer rules: one guardian with the permission per child is
// enough, or every such guardian of the child must declare.
const (
	DeclarationSignersAny = "any"
	DeclarationSignersAll = "all"
)

// Declaration actions (mirrors the submissions' action CHECK).
const (
	DeclarationActionAgreed   = "agreed"
	DeclarationActionDeclined = "declined"
	DeclarationActionRevoked  = "revoked"
)

// DeclarationMethodSimpleElectronic is the only procedure moto offers: a
// simple electronic declaration from the signed-in parent account. It is not
// a qualified electronic signature and replaces no statutory written form.
const DeclarationMethodSimpleElectronic = "simple_electronic"

// Per-child states of a declaration for its current version.
const (
	DeclarationStateOpen     = "open"
	DeclarationStatePartial  = "partial"
	DeclarationStateAgreed   = "agreed"
	DeclarationStateDeclined = "declined"
	DeclarationStateRevoked  = "revoked"
	DeclarationStateNoSigner = "no_signer"
	DeclarationStateExpired  = "expired"
)

// ErrDeclarationVersionConflict is returned by the store when a version with
// the same number was written concurrently.
var ErrDeclarationVersionConflict = errors.New("users: declaration version already exists")

// AnnouncementDeclarationSettings are the declaration columns of an
// announcement. Kind is empty for every other delivery mode.
type AnnouncementDeclarationSettings struct {
	Kind             string `bun:"kind,nullzero"`
	Signers          string `bun:"signers,nullzero"`
	Revocable        bool   `bun:"revocable,notnull"`
	RequiresPassword bool   `bun:"requires_password,notnull"`
}

// IsDeclaration reports whether the announcement is an Erklärung.
func (a *ParentAnnouncement) IsDeclaration() bool {
	return a.DeliveryMode == ParentAnnouncementDeliveryDeclaration
}

// DeclarationAttachmentDigest identifies one attachment of a frozen version by
// its content. The SHA-256 lets anyone holding the file check that it is the
// one the guardian saw.
type DeclarationAttachmentDigest struct {
	AttachmentID int64  `json:"attachment_id"`
	Filename     string `json:"filename"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
}

// DeclarationVersion is the frozen wording of one publication of an
// Erklärung.
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

// DeclarationSubmission is one declared action. Rows are never changed.
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

// DeclarationChild is one child an Erklärung reaches.
type DeclarationChild struct {
	AnnouncementID int64
	StudentID      int64
	FirstName      string
	LastName       string
	SchoolClass    string
}

// DeclarationSigner is one guardian of a reached child who may declare for
// it: a linked account with an active membership, parent_portal.access and
// parent_portal.declarations.submit on exactly this relationship.
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

// DeclarationSignerContext is the relationship a submission is made under,
// read and share-locked inside the write transaction.
type DeclarationSignerContext struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
	GuardianRole      string
}

// ParentDeclarationRepository is the declaration part of the announcement
// repository.
type ParentDeclarationRepository interface {
	// LatestDeclarationVersion returns the newest frozen version, or nil.
	LatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*DeclarationVersion, error)
	// ListDeclarationVersions returns every version, newest first.
	ListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*DeclarationVersion, error)
	// LatestDeclarationVersions batches LatestDeclarationVersion for a feed.
	LatestDeclarationVersions(ctx context.Context, announcementIDs []int64) (map[int64]*DeclarationVersion, error)
	// InsertDeclarationVersion writes a new version.
	InsertDeclarationVersion(ctx context.Context, version *DeclarationVersion) error
	// DeclarationSettings batches the declaration columns for a feed.
	DeclarationSettings(ctx context.Context, announcementIDs []int64) (map[int64]AnnouncementDeclarationSettings, error)
	// CountDeclarationSubmissions counts every submission of an announcement.
	CountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error)
	// ListDeclarationSubmissions returns the whole history, newest first.
	ListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*DeclarationSubmission, error)
	// ListDeclarationSubmissionsForStudents batches the history for a feed.
	ListDeclarationSubmissionsForStudents(ctx context.Context, announcementIDs, studentIDs []int64) ([]*DeclarationSubmission, error)
	// LockDeclarationChild serializes submissions for one child of one
	// announcement until the transaction ends.
	LockDeclarationChild(ctx context.Context, announcementID, studentID int64) error
	// InsertDeclarationSubmission appends one submission.
	InsertDeclarationSubmission(ctx context.Context, submission *DeclarationSubmission) error
	// DeclarationChildren returns every child the announcement reaches.
	DeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*DeclarationChild, error)
	// DeclarationSigners returns every guardian who may declare for the given
	// children of the school.
	DeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*DeclarationSigner, error)
	// DeclarationChildrenForAccount returns the account's reached children per
	// declaration announcement (cross-school, admin transaction).
	DeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64) ([]*DeclarationChild, error)
	// DeclarationSignersForStudents is DeclarationSigners across schools.
	DeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*DeclarationSigner, error)
	// HoldDeclarationSigner reads and share-locks the relationship that
	// authorizes accountID to declare for studentID, or returns nil.
	HoldDeclarationSigner(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (*DeclarationSignerContext, error)
	// HoldHistoricalDeclarationSigner is the revocation-only counterpart for
	// an account's own recorded declaration after an activity enrollment ended
	// or its declaration permission was removed.
	HoldHistoricalDeclarationSigner(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (*DeclarationSignerContext, error)
	// ReadOpenDeclarations returns the live, open Erklärungen of the schools
	// that the account has already opened, with their deadline.
	ReadOpenDeclarations(ctx context.Context, accountID int64, tenantIDs []int64) (map[int64]*time.Time, error)
}

// AnnouncementFeedDeclaration is the declaration part of a feed item, built
// by the parent flow for the reading account.
type AnnouncementFeedDeclaration struct {
	Kind             string
	Signers          string
	Revocable        bool
	RequiresPassword bool
	Deadline         *time.Time
	Closed           bool
	Version          *DeclarationVersion
	Children         []*AnnouncementFeedDeclarationChild
}

// AnnouncementFeedDeclarationChild is one child of the reading account.
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

// DeclarationSignerState is one guardian with their latest action on the
// current version (nil = nothing yet).
type DeclarationSignerState struct {
	AccountID   int64
	FirstName   string
	LastName    string
	Action      *string
	SubmittedAt *time.Time
}

// The format values version the canonical encodings below. A change to one
// encoding must bump only its format, never reinterpret stored hashes.
const (
	declarationContentHashFormat = 1
	declarationRecordHashFormat  = 2
)

type canonicalDeclarationVersion struct {
	Format      int                           `json:"format"`
	Title       string                        `json:"title"`
	Body        string                        `json:"body"`
	Kind        string                        `json:"kind"`
	Attachments []DeclarationAttachmentDigest `json:"attachments"`
}

// CanonicalContent is the canonical JSON of what the guardian is shown:
// title, body, kind and every attachment's name, type, size and content
// digest, in attachment-id order. Its SHA-256 is the version's content hash;
// two versions with the same hash showed the same content.
func (v *DeclarationVersion) CanonicalContent() []byte {
	attachments := append([]DeclarationAttachmentDigest(nil), v.Attachments...)
	sort.Slice(attachments, func(i, j int) bool { return attachments[i].AttachmentID < attachments[j].AttachmentID })
	if attachments == nil {
		attachments = []DeclarationAttachmentDigest{}
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
}

// CanonicalRecord is the canonical JSON of the submission, including the
// content hash of the version it refers to. Its SHA-256 is the record hash,
// which makes a later change to a stored row detectable; it proves neither
// the signer's identity nor a trusted time on its own. AccountID is excluded:
// account deletion clears that nullable foreign key, while the frozen signer
// identity remains in this record.
func (s *DeclarationSubmission) CanonicalRecord() []byte {
	return canonicalJSON(canonicalDeclarationSubmission{
		Format: declarationRecordHashFormat, TenantID: s.TenantID, AnnouncementID: s.AnnouncementID,
		VersionID: s.VersionID, ContentHash: s.ContentHash, StudentID: s.StudentID,
		GuardianProfileID: s.GuardianProfileID, SignerName: s.SignerName, GuardianRole: s.GuardianRole,
		Action: s.Action, Method: s.Method, PasswordConfirmed: s.PasswordConfirmed,
		SubmittedAt: s.SubmittedAt.UTC().Format(time.RFC3339Nano),
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
