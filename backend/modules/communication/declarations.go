package communication

import (
	"context"
	"errors"
	"time"
)

// Erklärungen (#3430): the public staff view of a declaration and the German
// wording of its values. The rules that derive a child's state live with the
// flows that apply them (services/parentmessaging).

// Declaration vocabulary. The strings are the database values.
const (
	DeclarationKindConsent         = "consent"
	DeclarationKindAcknowledgement = "acknowledgement"

	DeclarationSignersAny = "any"
	DeclarationSignersAll = "all"

	DeclarationActionAgreed       = "agreed"
	DeclarationActionDeclined     = "declined"
	DeclarationActionAcknowledged = "acknowledged"
	DeclarationActionRevoked      = "revoked"

	DeclarationStateOpen         = "open"
	DeclarationStatePartial      = "partial"
	DeclarationStateAgreed       = "agreed"
	DeclarationStateDeclined     = "declined"
	DeclarationStateAcknowledged = "acknowledged"
	DeclarationStateRevoked      = "revoked"
	DeclarationStateNoSigner     = "no_signer"
	DeclarationStateExpired      = "expired"
)

var (
	// ErrDeclarationHasSubmissions refuses deleting a declaration somebody
	// already declared on: the proof must not disappear with it.
	ErrDeclarationHasSubmissions = errors.New("declaration: guardians already declared on it")
	// ErrNotDeclaration reports a declaration read on another mode.
	ErrNotDeclaration = errors.New("declaration: announcement is not an Erklärung")
)

// ParentDeclarationSettings are the Erklärung settings on the announcement
// input and output. Kind is empty for other delivery modes.
type ParentDeclarationSettings struct {
	Kind             string
	Signers          string
	Revocable        bool
	RequiresPassword bool
}

// DeclarationAttachment is one frozen attachment digest.
type DeclarationAttachment struct {
	Filename    string
	ContentType string
	SizeBytes   int64
	SHA256      string
}

// DeclarationVersion is one frozen publication.
type DeclarationVersion struct {
	ID          int64
	VersionNo   int
	Title       string
	Body        string
	Kind        string
	ContentHash string
	PublishedAt time.Time
	Attachments []DeclarationAttachment
	// IntegrityOK is false when the stored content no longer matches its hash.
	IntegrityOK bool
}

// DeclarationSubmissionRecord is one stored declaration.
type DeclarationSubmissionRecord struct {
	ID                int64
	StudentID         int64
	StudentFirstName  string
	StudentLastName   string
	AccountID         *int64
	SignerName        string
	GuardianRole      string
	Action            string
	Method            string
	PasswordConfirmed bool
	VersionNo         int
	ContentHash       string
	RecordHash        string
	SubmittedAt       time.Time
	// IntegrityOK is false when the stored row no longer matches its hash.
	IntegrityOK bool
}

// DeclarationSignerStatus is one eligible guardian of a child.
type DeclarationSignerStatus struct {
	AccountID   int64
	FirstName   string
	LastName    string
	Action      *string
	SubmittedAt *time.Time
}

// DeclarationChildStatus is one reached child.
type DeclarationChildStatus struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
	State       string
	Signers     []DeclarationSignerStatus
}

// DeclarationSummary counts children per state.
type DeclarationSummary struct {
	ChildrenTotal int
	ByState       map[string]int
}

// ParentDeclarationStatus is the staff view of an Erklärung.
type ParentDeclarationStatus struct {
	Title            string
	Settings         ParentDeclarationSettings
	Deadline         *time.Time
	CurrentVersion   *DeclarationVersion
	Versions         []DeclarationVersion
	Summary          DeclarationSummary
	Children         []DeclarationChildStatus
	Submissions      []DeclarationSubmissionRecord
	GeneratedAt      time.Time
	IntegrityAllGood bool
}

// ParentDeclarationCapability is the staff side of Erklärungen.
type ParentDeclarationCapability interface {
	ParentDeclarationStatus(ctx context.Context, announcementID int64) (*ParentDeclarationStatus, error)
}

// DeclarationActionLabel is the German wording of an action in a proof.
func DeclarationActionLabel(action string) string {
	switch action {
	case DeclarationActionAgreed:
		return "Zugestimmt"
	case DeclarationActionDeclined:
		return "Abgelehnt"
	case DeclarationActionAcknowledged:
		return "Zur Kenntnis genommen"
	case DeclarationActionRevoked:
		return "Zustimmung widerrufen"
	default:
		return action
	}
}

// DeclarationMethodLabel names the procedure in a proof. It deliberately
// says what the procedure is and what it is not.
func DeclarationMethodLabel(method string) string {
	if method == "simple_electronic" {
		return "Einfache elektronische Erklärung im angemeldeten Eltern-Konto"
	}
	return method
}

// GuardianRoleLabel is the German wording of the relationship a guardian
// declared under.
func GuardianRoleLabel(role string) string {
	switch role {
	case "primary_guardian":
		return "Hauptberechtigt"
	case "legal_guardian":
		return "Erziehungsberechtigt"
	case "co_guardian":
		return "Mitberechtigt"
	case "custom":
		return "Individuell"
	default:
		return role
	}
}
