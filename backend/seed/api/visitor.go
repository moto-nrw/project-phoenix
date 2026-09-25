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

// visitorDisplayName returns the name a seeded person is created with. The
// internal keys keep the seed names. A seed person who happens to share the
// visitor's name takes the displaced name, so the visitor appears once.
func visitorDisplayName(visitor string, isVisitor bool, first, last, displacedFirst, displacedLast string) (string, string) {
	visitor = strings.Join(strings.Fields(visitor), " ")
	if visitor == "" {
		return first, last
	}
	visitorFirst, visitorLast, ok := strings.Cut(visitor, " ")
	if !ok {
		// A single name takes the family name of the visitor's child, in the
		// OGS app as in the parents app, so the visitor has one name.
		visitorLast = DemoGuardians[visitorGuardianIndex].LastName
	}
	if isVisitor {
		return visitorFirst, visitorLast
	}
	if strings.EqualFold(first, visitorFirst) && strings.EqualFold(last, visitorLast) {
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
