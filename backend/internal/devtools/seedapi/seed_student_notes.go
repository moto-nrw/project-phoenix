package api

import (
	"context"
	"fmt"
)

// seedStudentNotesStep fills the note card ("Notizen") of a few demo children
// (#3632). A card that is empty on every dev machine is a screen nobody
// reviews, and this one has more to show than a list: the durable hints that
// replaced the Betreuernotizen field, dated entries, a note about a child in
// one activity, and all three audiences next to each other.
type seedStudentNotesStep struct{}

func (seedStudentNotesStep) Name() string { return "Seeding student notes" }

func (seedStudentNotesStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("student note prerequisites not available")
	}
	today := todaySeedDate()
	groupID := rt.FixedSeeder.groupIDs["sternengruppe"]

	// Index 0 carries the full card: a durable hint, two dated entries and one
	// note that only the group leadership reads. The others get a single note
	// each, so a reviewer sees both a rich and a sparse card.
	notes := []struct {
		studentIndex int
		note         map[string]any
	}{
		{0, map[string]any{
			"kind":       "permanent",
			"visibility": "all_staff",
			"category":   "general",
			"body":       "Braucht morgens etwas Zeit zum Ankommen. Ein ruhiger Platz hilft.",
		}},
		{0, map[string]any{
			"kind":         "journal",
			"visibility":   "all_staff",
			"category":     "positive",
			"body":         "Hat heute in der Hausaufgabenzeit einem anderen Kind geholfen.",
			"subject_date": today.String(),
		}},
		{0, map[string]any{
			"kind":         "journal",
			"visibility":   "care_team",
			"category":     "conversation",
			"body":         "Kurzes Gespräch über den Streit von gestern. Es ist geklärt.",
			"subject_date": today.AddDays(-1).String(),
		}},
		{1, map[string]any{
			"kind":       "permanent",
			"visibility": "all_staff",
			"category":   "general",
			"body":       "Geht freitags mit der Nachbarin nach Hause.",
		}},
		{2, map[string]any{
			"kind":         "journal",
			"visibility":   "care_team",
			"category":     "parent_contact",
			"body":         "Mutter angerufen: Das Kind kommt nächste Woche später.",
			"subject_date": today.String(),
		}},
	}
	if groupID != 0 {
		notes = append(notes, struct {
			studentIndex int
			note         map[string]any
		}{0, map[string]any{
			"kind":               "journal",
			"visibility":         "group_leads",
			"category":           "incident",
			"body":               "Bitte in der Gruppenleitung besprechen: wiederholte Konflikte in der Freispielzeit.",
			"subject_date":       today.String(),
			"education_group_id": fmt.Sprintf("%d", groupID),
		}})
	}

	created := 0
	for _, entry := range notes {
		studentID, ok := rt.FixedSeeder.studentIDByIndex[entry.studentIndex]
		if !ok {
			return fmt.Errorf("student note demo index %d not available", entry.studentIndex)
		}
		if _, err := rt.Client.Post(fmt.Sprintf("/api/students/%d/notes", studentID), entry.note); err != nil {
			return fmt.Errorf("create student note for index %d: %w", entry.studentIndex, err)
		}
		created++
	}

	fmt.Printf("  %d student notes created\n", created)
	return nil
}
