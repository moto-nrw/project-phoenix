package analytics

import (
	"context"
	"strconv"
)

// PseudonymPrefix starts every pseudonymous user ID. The browser filter drops
// an event that carries one outside the Analyse-Freigabe.
const PseudonymPrefix = "pseudo_"

// pseudonymLength is how many hex digits of the fingerprint the ID keeps
// (128 bits).
const pseudonymLength = 32

// Fingerprint hashes content to a stable hex identity. The composition root
// passes the Security Runtime's SHA-256 fingerprint; the browser hashes the
// same input with SHA-256 (frontend/src/lib/analytics-pseudonym.ts), so page
// views, recordings, and backend events of one person meet in one profile.
type Fingerprint func(content []byte) string

// pseudonymContent is the input of the pseudonymous ID: school and account,
// versioned, so the same account at another school is another person.
func pseudonymContent(schoolID, accountID int64) []byte {
	return []byte("moto-analytics:v1:" +
		strconv.FormatInt(schoolID, 10) + ":" + strconv.FormatInt(accountID, 10))
}

// pseudonymousID is the user ID of an OGS account while its school has the
// Analyse-Freigabe (#3603): a hash of school and account, never the account
// ID itself. Empty when the fingerprint cannot produce one.
func pseudonymousID(fingerprint Fingerprint, schoolID, accountID int64) string {
	if fingerprint == nil || schoolID <= 0 || accountID <= 0 {
		return ""
	}
	hash := fingerprint(pseudonymContent(schoolID, accountID))
	if len(hash) < pseudonymLength {
		return ""
	}
	return PseudonymPrefix + hash[:pseudonymLength]
}

// person is an OGS account of a school with Analyse-Freigabe.
type person struct {
	schoolID  int64
	accountID int64
}

type personKey struct{}

// withPerson marks the request's account as a pseudonymous person. Only the
// core-action middleware sets it, and only for an OGS request of a school
// with Analyse-Freigabe; the tracker then sends the event under the
// pseudonymous ID with a person profile. Every other event stays anonymous.
func withPerson(ctx context.Context, schoolID, accountID int64) context.Context {
	return context.WithValue(ctx, personKey{}, person{schoolID: schoolID, accountID: accountID})
}

func personFromContext(ctx context.Context) (person, bool) {
	if ctx == nil {
		return person{}, false
	}
	p, ok := ctx.Value(personKey{}).(person)
	return p, ok
}
