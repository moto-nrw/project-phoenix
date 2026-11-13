// Package schoolsearchview is the public contract of the school search
// projection (ADR 0231, #6954): one search over the children, their contact
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
	"regexp"
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
	// MinFullGermanDigits is the length from which digits starting with 49
	// but typed without + or 00 read as a German number with its country
	// code: 49 and a number of at least nine digits.
	MinFullGermanDigits = 11
	// MinStoredPhoneDigits and MaxExtensionDigits bound the extension rule:
	// a query that is a stored number of at least MinStoredPhoneDigits key
	// digits plus up to MaxExtensionDigits more ("0221 5550001-12") finds
	// that number. contactsByPhoneSQL spells both numbers out.
	MinStoredPhoneDigits = 7
	MaxExtensionDigits   = 5
	// ChildLimit and ContactLimit bound the person lists; OtherLimit the
	// staff, rooms and groups.
	ChildLimit   = 8
	ContactLimit = 8
	OtherLimit   = 5
	// MaxMatchTextRunes bounds the text a hit shows for its reason: a long
	// parent note comes as the part around the match, with "…" where it was
	// cut.
	MaxMatchTextRunes = 120
)

// ErrInvalidQuery reports a query outside MinQueryRunes..MaxQueryRunes.
var ErrInvalidQuery = fmt.Errorf("school search: the query must have between %d and %d characters", MinQueryRunes, MaxQueryRunes)

// Scope is what the account may read without the search. Every field maps to
// the permission check of the page the hit leads to.
type Scope struct {
	// Children opens names, class and group (users:read or students:read,
	// tenant staff or admin).
	Children bool `json:"children"`
	// ChildDetails opens the parent notes and the registration answers of a
	// child (users:read, tenant staff or admin).
	ChildDetails bool `json:"child_details"`
	// Contacts opens the contact persons with their phone numbers and e-mail
	// addresses (users:read, tenant staff or admin).
	Contacts bool `json:"contacts"`
	// Staff opens the staff directory (users:read, staff:manage or
	// staff:stammdaten).
	Staff bool `json:"staff"`
	// Rooms opens the rooms (rooms:read).
	Rooms bool `json:"rooms"`
	// Groups opens the OGS groups (groups:read and a child read).
	Groups bool `json:"groups"`
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
// question), Text the matched value as stored; a parent note comes as the
// part around the match, at most MaxMatchTextRunes characters.
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
// kind found nothing or lies outside the Scope. Scope is the account's own
// scope, the same for every query: the dialog explains from it what this
// account can find, and it says nothing about the hits.
type Result struct {
	Children []Child   `json:"children"`
	Contacts []Contact `json:"contacts"`
	Staff    []Staff   `json:"staff"`
	Rooms    []Room    `json:"rooms"`
	Groups   []Group   `json:"groups"`
	Scope    Scope     `json:"scope"`
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
// only digits and separators (isPhoneSeparator), and its key has at least
// MinPhoneDigits digits once the prefixes are gone: "0170" is the area code
// 170, too short to search by.
//
// The key is PhoneNumberKey, the number without its trunk or country prefix,
// so every way of writing the same German number meets: "+49 170 1234567",
// "0049 170 1234567", "0170/1234567" and "0170-123 45 67" all become
// "1701234567". A partial number such as the last digits matches as a part of
// a stored key; a number typed with an extension matches the stored number
// without it (contactsByPhoneSQL).
func PhoneSearchKey(text string) string {
	for _, r := range text {
		if (r < '0' || r > '9') && !isPhoneSeparator(r) {
			return ""
		}
	}
	key := PhoneNumberKey(text)
	if len(key) < MinPhoneDigits {
		return ""
	}
	return key
}

// isPhoneSeparator reports the characters people put between the digits of a
// number: every Unicode space (a non-breaking space from a mail signature, a
// tab), every dash (hyphen, en dash, non-breaking hyphen) and the minus sign,
// the plus and / . ( ). The frontend's phoneSearchKey uses the same set.
func isPhoneSeparator(r rune) bool {
	return unicode.Is(unicode.White_Space, r) || unicode.Is(unicode.Pd, r) || r == '\u2212' ||
		strings.ContainsRune("+/.()", r)
}

// trunkZeroMarker is the bracketed trunk zero of an international number,
// "+49 (0)170 …" or "+43 (0)1 …": it is not dialled from abroad.
var trunkZeroMarker = regexp.MustCompile(`\( *0 *\)`)

// PhoneNumberKey is the key of any text that holds a phone number: its ASCII
// digits without the bracketed trunk zero, the international 00, the German
// country code 49 and the German trunk zero. A number of another country keeps
// its country code ("+43 1 234" becomes "431234"), which still finds itself.
//
// A text counts as international when its first digit or plus is a plus, or
// its digits start with 00. Without either, a leading 49 is dropped only from
// a number of MinFullGermanDigits digits or more ("49 221 5550001"): a shorter
// one may be the end of a number and stays as typed.
//
// phoneKeyOfStoredNumber in the Postgres adapter computes the same key for
// stored numbers; TestPhoneSearchKeySQLMatchesGo holds both to the same cases.
func PhoneNumberKey(text string) string {
	text = trunkZeroMarker.ReplaceAllString(text, "")
	var digits strings.Builder
	plusFirst, decided := false, false
	for _, r := range text {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
			decided = true
		case r == '+' && !decided:
			plusFirst, decided = true, true
		}
	}
	key := digits.String()
	international := plusFirst || strings.HasPrefix(key, "00")
	key = strings.TrimPrefix(key, "00")
	if strings.HasPrefix(key, "49") && (international || len(key) >= MinFullGermanDigits) {
		key = key[2:]
	}
	if !international || strings.HasPrefix(key, "0") {
		key = strings.TrimLeft(key, "0")
	}
	return key
}
