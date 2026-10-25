package api

import (
	"context"
	"fmt"
	"time"
)

// seedFamilyConversationTemplates are messages from further families; %[1]s
// is the child's first name. Some stay unanswered, so the staff inbox has
// unread conversations.
var seedFamilyConversationTemplates = []struct {
	position        int
	message, answer string
}{
	{1, "Guten Morgen! %[1]s hat heute einen Zahnarzttermin. Ich hole %[1]s um 14:30 Uhr ab.", "Danke für die Nachricht. Wir sagen %[1]s rechtzeitig Bescheid."},
	{8, "%[1]s hat seit gestern etwas Husten. Bitte heute nicht so lange draußen spielen lassen.", ""},
	{15, "Hallo, die Turnschuhe von %[1]s sind weg. Liegen sie vielleicht noch in der Turnhalle?", "Ja, wir haben sie gefunden. Sie stehen jetzt im Fach von %[1]s."},
	{22, "Darf %[1]s am Donnerstag mit zu Mia nach Hause? Mias Mutter holt beide um 15 Uhr ab.", ""},
	{29, "Ab nächster Woche hat %[1]s montags Schwimmen im Verein und geht dann schon um 15 Uhr.", "Danke, das tragen wir ein."},
}

// seedFamilyConversations writes the conversations of further families, so
// the staff inbox holds about ten of them. Without family app accounts
// (DeferHistory) it writes none.
func seedFamilyConversations(ctx context.Context, rt *Runtime) (int, error) {
	written := 0
	for _, c := range seedFamilyConversationTemplates {
		parent, ok := familyAccountAt(rt, c.position)
		if !ok {
			continue
		}
		name := demoChildFirstName(rt, parent.StudentIDs[0])
		answer := ""
		if c.answer != "" {
			answer = fmt.Sprintf(c.answer, name)
		}
		if err := seedFamilyConversation(ctx, rt, parent, fmt.Sprintf(c.message, name), answer); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}

// seedUrgentPickupChanges files two pickup changes for today, which the
// staff request inbox marks as urgent.
func seedUrgentPickupChanges(ctx context.Context, rt *Runtime) (int, error) {
	return seedUrgentPickupChangesOn(ctx, rt, todaySeedDate())
}

// seedUrgentPickupChangesOn files them for the given day. On a weekend there
// is no care day and nothing to change.
func seedUrgentPickupChangesOn(ctx context.Context, rt *Runtime, today seedDate) (int, error) {
	if today.Weekday() == time.Saturday || today.Weekday() == time.Sunday {
		return 0, nil
	}
	requests := []struct {
		position           int
		pickupTime, reason string
	}{
		{4, "14:00", "Wir haben heute Nachmittag einen Termin beim Kinderarzt."},
		{19, "14:45", "Heute holt die Oma ab. Wir fahren danach direkt zu ihr."},
	}
	filed := 0
	for _, r := range requests {
		parent, ok := familyAccountInCare(rt, r.position)
		if !ok {
			continue
		}
		auth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
		if err != nil {
			return filed, fmt.Errorf("login parent %s: %w", parent.Email, err)
		}
		if _, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/parent/me/children/%d/care-exception", parent.StudentIDs[0]), map[string]any{
			"date": today.String(), "pickup_time": r.pickupTime, "reason": r.reason,
		}); err != nil {
			return filed, fmt.Errorf("create urgent pickup change: %w", err)
		}
		filed++
	}
	return filed, nil
}

// seedFamilyMealWeekdays are the regular lunch days the families choose.
var seedFamilyMealWeekdays = [][]int{{1, 2, 3, 4, 5}, {1, 2, 3, 4}, {1, 2, 4}, {2, 3, 4, 5}}

// seedFamilyMealParticipation registers every third family app account's
// child for lunch, about twenty children, so the kitchen's day list is not
// a single child. The children at the end of the cohort stay out: two of
// them leave care soon.
func seedFamilyMealParticipation(ctx context.Context, rt *Runtime) (int, error) {
	registered := 0
	for i := 0; i < len(rt.FamilyAccounts) && i < 60; i += 3 {
		parent := rt.FamilyAccounts[i]
		auth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
		if err != nil {
			return registered, fmt.Errorf("login parent %s: %w", parent.Email, err)
		}
		if _, err := rt.Client.PutWithAuth(auth, fmt.Sprintf("/parent/me/children/%d/meal-participation", parent.StudentIDs[0]), map[string]any{
			"weekdays": seedFamilyMealWeekdays[registered%len(seedFamilyMealWeekdays)],
		}); err != nil {
			return registered, fmt.Errorf("seed family meal participation: %w", err)
		}
		registered++
	}
	return registered, nil
}

// familyAccountAt returns the family app account at the position, if there
// is one with a child.
func familyAccountAt(rt *Runtime, position int) (ParentCredentials, bool) {
	if position >= len(rt.FamilyAccounts) || len(rt.FamilyAccounts[position].StudentIDs) == 0 {
		return ParentCredentials{}, false
	}
	return rt.FamilyAccounts[position], true
}

// familyAccountInCare returns the first family app account from the
// position on whose child the seed does not report sick, so a pickup change
// for today concerns a child who is in care.
func familyAccountInCare(rt *Runtime, position int) (ParentCredentials, bool) {
	sick := map[int]bool{}
	for _, index := range sickDemoCandidates() {
		sick[index] = true
	}
	for ; position < len(rt.FamilyAccounts); position++ {
		parent, ok := familyAccountAt(rt, position)
		if ok && !sick[demoStudentIndex(rt, parent.StudentIDs[0])] {
			return parent, true
		}
	}
	return ParentCredentials{}, false
}

// demoStudentIndex is the DemoStudents index of a seeded child, or -1.
func demoStudentIndex(rt *Runtime, studentID int64) int {
	if rt.FixedSeeder != nil {
		for index, id := range rt.FixedSeeder.studentIDByIndex {
			if id == studentID {
				return index
			}
		}
	}
	return -1
}

// demoChildFirstName names a demo child in a family's message; a child the
// seed does not know is "unser Kind".
func demoChildFirstName(rt *Runtime, studentID int64) string {
	if index := demoStudentIndex(rt, studentID); index >= 0 && index < len(DemoStudents) {
		return DemoStudents[index].FirstName
	}
	return "unser Kind"
}
