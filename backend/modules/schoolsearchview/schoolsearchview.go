// Package schoolsearchview is the public contract of the school search
// projection (ADR 0230, #6954): one search over the children, their contact
// persons, the staff, the rooms and the OGS groups of the school in the
// caller's tenant transaction. The projection owns no data and never writes.
//
// The caller decides what the account may read and passes it as a Scope. The
// projection searches only the kinds and fields the Scope opens: an account
// without contact access gets no contact hit and no child found through a
// phone number, e-mail or registration answer, and the answer shape stays the
// same either way.
package schoolsearchview

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// Limits of one search. A result list stops at its limit; the dialog shows a
// short list per kind, never a full directory.
const (
	// MinQueryRunes is the shortest query that searches. One letter would
	// list half the school.
	MinQueryRunes = 2
	// MaxQueryRunes caps the query; longer input is refused, not cut.
	MaxQueryRunes = 100
	// MinPhoneDigits is the fewest digits a phone query needs. Fewer digits
	// would match nearly every number.
	MinPhoneDigits = 4
	// ChildLimit and ContactLimit bound the person lists; OtherLimit the
	// staff, rooms and groups.
	ChildLimit   = 8
	ContactLimit = 8
	OtherLimit   = 5
)

// ErrInvalidQuery reports a query outside MinQueryRunes..MaxQueryRunes.
var ErrInvalidQuery = fmt.Errorf("school search: the query must have between %d and %d characters", MinQueryRunes, MaxQueryRunes)

// Scope is what the account may read without the search. Every field maps to
// the permission check of the page the hit leads to.
type Scope struct {
	// Children opens names, class and group (users:read or students:read,
	// tenant staff or admin).
	Children bool
	// ChildDetails opens the parent notes and the registration answers of a
	// child (users:read, tenant staff or admin).
	ChildDetails bool
	// Contacts opens the contact persons with their phone numbers and e-mail
	// addresses (users:read, tenant staff or admin).
	Contacts bool
	// Staff opens the staff directory (users:read, staff:manage or
	// staff:stammdaten).
	Staff bool
	// Rooms opens the rooms (rooms:read).
	Rooms bool
	// Groups opens the OGS groups (groups:read and a child read).
	Groups bool
}

// Query is one search request.
type Query struct {
	Text  string
	Scope Scope
}

// MatchField names where a hit matched, so the dialog can say why a row is
// there.
type MatchField string

const (
	MatchName             MatchField = "name"
	MatchClass            MatchField = "class"
	MatchGroup            MatchField = "group"
	MatchParentNote       MatchField = "parent_note"
	MatchEnrollmentAnswer MatchField = "enrollment_answer"
	MatchPhone            MatchField = "phone"
	MatchEmail            MatchField = "email"
	MatchContact          MatchField = "contact"
)

// Match is the reason of a hit. Label names the field (a registration
// question), Text the matched value as stored.
type Match struct {
	Field MatchField `json:"field"`
	Label string     `json:"label,omitempty"`
	Text  string     `json:"text,omitempty"`
}

// Child is a child hit.
type Child struct {
	ID          int64  `json:"id,string"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
	GroupName   string `json:"group_name,omitempty"`
	Match       Match  `json:"match"`
}

// ContactChild is a child a contact person belongs to.
type ContactChild struct {
	ID               int64  `json:"id,string"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	SchoolClass      string `json:"school_class"`
	RelationshipType string `json:"relationship_type"`
}

// Contact is a contact person hit with the children it belongs to.
type Contact struct {
	ID        int64          `json:"id,string"`
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	Match     Match          `json:"match"`
	Children  []ContactChild `json:"children"`
}

// Staff is a staff hit.
type Staff struct {
	ID        int64  `json:"id,string"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// Room is a room hit.
type Room struct {
	ID       int64  `json:"id,string"`
	Name     string `json:"name"`
	Building string `json:"building,omitempty"`
}

// Group is an OGS group hit.
type Group struct {
	ID   int64  `json:"id,string"`
	Name string `json:"name"`
}

// Result is the answer of one search. Every list is present, empty when the
// kind found nothing or lies outside the Scope.
type Result struct {
	Children []Child   `json:"children"`
	Contacts []Contact `json:"contacts"`
	Staff    []Staff   `json:"staff"`
	Rooms    []Room    `json:"rooms"`
	Groups   []Group   `json:"groups"`
}

// Search is the projection's capability.
type Search interface {
	SearchSchool(ctx context.Context, query Query) (Result, error)
}

// StudentAnswerSearch is the consumer-owned port to Enrollment's answers of
// the school's children (enrollment.StudentAnswers). The root binds it.
type StudentAnswerSearch interface {
	SearchStudentAnswers(ctx context.Context, text string, limit int) ([]enrollment.StudentAnswerMatch, error)
}

// ValidateQueryText trims the query and checks its length.
func ValidateQueryText(text string) (string, error) {
	text = strings.TrimSpace(text)
	n := utf8.RuneCountInString(text)
	if n < MinQueryRunes || n > MaxQueryRunes {
		return "", ErrInvalidQuery
	}
	return text, nil
}

// PhoneSearchKey reads a query as a phone number and returns the digits it is
// compared with, or "" when the query is no phone number. A phone query holds
// only digits and the separators people type (space, +, -, /, ., brackets)
// and at least MinPhoneDigits digits.
//
// The key is the number without its trunk or country prefix, so every way of
// writing the same German number meets: "+49 170 1234567", "0049 170
// 1234567", "0170/1234567" and "0170-123 45 67" all become "1701234567".
// "+49 (0)170 …" drops the bracketed 0 too. A number of another country keeps
// its country code ("+43 1 234" becomes "431234"), which still finds itself.
// A partial number such as the last digits matches as a part of the key.
//
// The SQL expression phoneSearchKeySQL in the Postgres adapter computes the
// same key for stored numbers; a test holds both to the same cases.
func PhoneSearchKey(text string) string {
	text = strings.TrimSpace(text)
	digits := make([]rune, 0, len(text))
	for _, r := range text {
		switch {
		case unicode.IsDigit(r) && r < utf8.RuneSelf:
			digits = append(digits, r)
		case strings.ContainsRune(" +-/.()", r):
		default:
			return ""
		}
	}
	if len(digits) < MinPhoneDigits {
		return ""
	}
	return phoneKeyFromDigits(text, string(digits))
}

// phoneKeyFromDigits removes the German prefixes from the digits of text.
func phoneKeyFromDigits(text, digits string) string {
	international := strings.HasPrefix(text, "+") || strings.HasPrefix(digits, "00")
	digits = strings.TrimPrefix(digits, "00")
	if international && strings.HasPrefix(digits, "49") {
		digits = strings.TrimPrefix(digits, "49")
	}
	if !international || strings.HasPrefix(digits, "0") {
		digits = strings.TrimLeft(digits, "0")
	}
	return digits
}
