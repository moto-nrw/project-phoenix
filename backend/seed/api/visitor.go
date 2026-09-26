package api

import "strings"

// The prospect of a public demo school takes the place of one caregiver and
// one parent (#3463). Julia Klein leads the Sternengruppe and has shifts;
// Sabine Schneider has a parent account and a child in that group. Only the
// name changes: every account keeps its synthetic address, so the prospect's
// real address never becomes an account or a guardian contact.
const (
	visitorStaffIndex    = 10
	visitorGuardianIndex = 0
)

// visitorName is the prospect's first and last name as the demo access
// stored them. Both are empty for a school without a visitor.
type visitorName struct{ first, last string }

func (v visitorName) empty() bool { return v.first == "" && v.last == "" }

// visitorDisplayName returns the name a seeded person is created with. The
// visitor's person takes the given first and last name unchanged. The
// internal keys keep the seed names. A seed person who happens to share the
// visitor's name takes the displaced name, so the visitor appears once.
func visitorDisplayName(visitor visitorName, isVisitor bool, first, last, displacedFirst, displacedLast string) (string, string) {
	if visitor.empty() {
		return first, last
	}
	if isVisitor {
		return visitor.first, visitor.last
	}
	if strings.EqualFold(first, visitor.first) && strings.EqualFold(last, visitor.last) {
		return displacedFirst, displacedLast
	}
	return first, last
}

// VisitorAccountID returns the account of the caregiver a visitor name
// replaced, so the demo access signs the prospect in as that person.
func VisitorAccountID(profile *SeedProfile) int64 {
	key := DemoStaff[visitorStaffIndex].FirstName + " " + DemoStaff[visitorStaffIndex].LastName
	for _, account := range profile.Credentials.Accounts.Betreuer {
		if account.Name == key {
			return account.AccountID
		}
	}
	return 0
}

// VisitorParentAccountID returns the parent account whose guardian carries
// the visitor's name (#3468). The demo role parent signs the prospect in as
// that parent, who has full parent portal rights for exactly one child.
func VisitorParentAccountID(profile *SeedProfile) int64 {
	guardian := DemoGuardians[visitorGuardianIndex]
	key := guardian.FirstName + " " + guardian.LastName
	for _, parent := range profile.Credentials.Parents {
		if parent.Name == key {
			return parent.AccountID
		}
	}
	return 0
}
