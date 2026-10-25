package care

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Signatures of pickup permissions (#4780). A new or changed pickup
// permission, a one-day collector and the permission to walk home alone (for
// one day or on weekdays) carry a simple electronic signature with a drawn
// signature. The flow decides who may sign and builds the German wording
// signed; the root-bound ledger of the Audit Platform stores the image
// append-only in the flow's unit of work, together with the SHA-256 of that
// wording. The request keeps the signature's id and the wording.

var (
	// ErrPickupSignatureRequired: the request needs a signature and carries
	// none. 400.
	ErrPickupSignatureRequired = errors.New("parent: the pickup permission requires a signature")
	// ErrPickupSignatureInvalid: the image or the typed name does not pass the
	// checks. 400.
	ErrPickupSignatureInvalid = errors.New("parent: the pickup signature is not valid")
	// ErrPickupPersonNotLinked: the chosen collector is not a contact person
	// of the child. 400.
	ErrPickupPersonNotLinked = errors.New("parent: the pickup person is not a contact of the child")
	// ErrPickupPermissionRequestRequired: a parent grants a pickup permission
	// only through a signed request the OGS confirms. 409.
	ErrPickupPermissionRequestRequired = errors.New("parent: a pickup permission is granted through a request")
	// ErrPickupPermissionUnchanged: the person already holds exactly this
	// permission. 409.
	ErrPickupPermissionUnchanged = errors.New("parent: the pickup permission is unchanged")
	// ErrPickupProofNotFound: no signed request of this guardian. 404.
	ErrPickupProofNotFound = errors.New("parent: pickup permission proof not found")
)

// PickupSignatureInput is a signature as the portal sends it.
type PickupSignatureInput struct {
	Method    string
	ImagePNG  string
	TypedName string
	// Statement is the server-generated wording rendered beside the signature
	// field. The submit path compares it byte-for-byte with its fresh wording
	// before it records the signature.
	Statement string
}

// PickupSignatureStatementInput identifies a declaration the portal renders
// before the parent signs it. The server owns the wording, including the
// child's current name, so the display and the later proof cannot drift.
type PickupSignatureStatementInput struct {
	Kind          string
	PersonName    string
	Weekdays      []int
	Date          calendar.Date
	DepartureMode string
}

const (
	PickupSignatureStatementPermission = "pickup_permission"
	PickupSignatureStatementDay        = "day_pickup"
	PickupSignatureStatementWeekly     = "weekly_alone"
)

// PickupSignatureEntry is one signature to store: the guardian, the child
// and the exact wording signed.
type PickupSignatureEntry struct {
	StudentID       int64
	SignerAccountID int64
	SignerName      string
	Statement       string
	SignedAt        time.Time
	Input           PickupSignatureInput
}

// PickupSignature is a stored signature as a proof shows it. Intact is the
// ledger's check of image and row, that it is a pickup permission signature
// of this child, and that the wording it was given on is the one stored with
// the request.
type PickupSignature struct {
	ID          int64
	Method      string
	TypedName   string
	SignerName  string
	PNG         []byte
	ImageSHA256 string
	SignedAt    time.Time
	Intact      bool
}

// PickupSignatureLedger is the flow's port to the shared signature store.
// Both calls run in the caller's tenant unit of work. Record checks the image
// and returns ErrPickupSignatureInvalid for a bad one; Signatures returns only
// rows of that school and checks each against the child and the wording.
type PickupSignatureLedger interface {
	RecordPickupSignature(context.Context, PickupSignatureEntry) (int64, error)
	PickupSignatures(ctx context.Context, studentID int64, statements map[int64]string) (map[int64]*PickupSignature, error)
}

// recordPickupSignature stores the signature for statement inside the
// caller's tenant transaction and returns the reference the request keeps.
func (s *Service) recordPickupSignature(txCtx context.Context, studentID, accountID int64, statement string, input *PickupSignatureInput) (*carerequests.SignedStatement, error) {
	if input == nil {
		return nil, ErrPickupSignatureRequired
	}
	if input.Statement != statement {
		return nil, ErrPickupSignatureInvalid
	}
	if s.PickupSignatures == nil {
		return nil, errors.New("parent: pickup signature ledger is not wired")
	}
	signer, _ := s.actorSnapshot(txCtx, accountID)
	name := ""
	if signer != nil {
		name = *signer
	}
	id, err := s.PickupSignatures.RecordPickupSignature(txCtx, PickupSignatureEntry{
		StudentID: studentID, SignerAccountID: accountID, SignerName: name,
		Statement: statement, SignedAt: s.now(), Input: *input,
	})
	if err != nil {
		return nil, err
	}
	return &carerequests.SignedStatement{SignatureID: id, Statement: statement}, nil
}

// PickupSignatureStatement returns the exact German declaration that the
// parent must see before signing. The write path rebuilds and compares it,
// because a contact or a child's name may have changed after this preview.
func (s *Service) PickupSignatureStatement(ctx context.Context, accountID, studentID int64, input PickupSignatureStatementInput) (string, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, GuardianPermissionPickupManage)
	if err != nil {
		return "", err
	}

	switch input.Kind {
	case PickupSignatureStatementPermission:
		name := strings.TrimSpace(input.PersonName)
		if name == "" {
			return "", ErrPickupSignatureInvalid
		}
		return PickupPermissionStatement(child.StudentName, name, normalizedWeekdays(input.Weekdays)), nil
	case PickupSignatureStatementDay:
		statement := DayPickupStatement(child.StudentName, input.Date, input.DepartureMode, strings.TrimSpace(input.PersonName))
		if statement == "" {
			return "", ErrPickupSignatureInvalid
		}
		return statement, nil
	case PickupSignatureStatementWeekly:
		statement := WeeklyAloneStatement(child.StudentName, normalizedWeekdays(input.Weekdays))
		if statement == "" {
			return "", ErrPickupSignatureInvalid
		}
		return statement, nil
	default:
		return "", ErrPickupSignatureInvalid
	}
}

var statementWeekdays = [...]string{"", "montags", "dienstags", "mittwochs", "donnerstags", "freitags"}
var statementDayNames = [...]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}

// statementDate is "Dienstag, 15.09.2026".
func statementDate(date calendar.Date) string {
	return statementDayNames[date.Weekday()] + ", " + date.Format("02.01.2006")
}

// statementWeekdayList is "montags und mittwochs", or "an allen Tagen" for nil.
func statementWeekdayList(weekdays []int) string {
	if len(weekdays) == 0 {
		return "an allen Tagen"
	}
	names := make([]string, 0, len(weekdays))
	for _, day := range weekdays {
		if day >= 1 && day <= 5 {
			names = append(names, statementWeekdays[day])
		}
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " und " + names[len(names)-1]
}

// PickupPermissionStatement is the wording of a pickup permission request.
func PickupPermissionStatement(childName, personName string, weekdays []int) string {
	return "Ich erlaube, dass " + personName + " das Kind " + childName + " " +
		statementWeekdayList(weekdays) + " von der OGS abholt."
}

// DayPickupStatement is the wording of a one-day request that needs a
// signature: a collector, or walking home alone. "" when it needs none.
func DayPickupStatement(childName string, date calendar.Date, departureMode, personName string) string {
	switch {
	case personName != "":
		return "Ich erlaube, dass " + personName + " das Kind " + childName + " am " +
			statementDate(date) + " von der OGS abholt."
	case departureMode == "alone":
		return "Ich erlaube, dass das Kind " + childName + " am " + statementDate(date) +
			" allein nach Hause geht."
	default:
		return ""
	}
}

// WeeklyAloneStatement is the wording of a weekly plan request that lets the
// child walk home alone on weekdays; "" when no weekday asks for it.
func WeeklyAloneStatement(childName string, aloneWeekdays []int) string {
	if len(aloneWeekdays) == 0 {
		return ""
	}
	return "Ich erlaube, dass das Kind " + childName + " " + statementWeekdayList(aloneWeekdays) +
		" allein nach Hause geht."
}
