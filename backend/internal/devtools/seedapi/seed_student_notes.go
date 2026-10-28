package api

import (
	"context"
	"fmt"
)

// seedStudentNotesStep fills the note card ("Notizen") of a few demo children
// (#3632). A card that is empty on every dev machine is a screen nobody
// reviews, and this one has more to show than a list: the durable hints that
// replaced the Betreuernotizen field, dated entries, a note about a child in
// one activity, and all three audiences next to each other. Two notes of index
// 0 are released for the school (#4797), so moto schule shows the Lehrkraft of
// its class a filled view, while the other notes stay with the OGS.
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
	notes := baseSeedStudentNotes(today)
	if groupID != 0 {
		notes = append(notes, seedStudentNoteEntry{0, map[string]any{
			"kind":               "journal",
			"visibility":         "group_leads",
			"category":           "incident",
			"body":               "Bitte in der Gruppenleitung besprechen: wiederholte Konflikte in der Freispielzeit.",
			"subject_date":       today.String(),
			"education_group_id": fmt.Sprintf("%d", groupID),
		}})
	}

	activityNote, err := seedActivityNote(rt, today.String())
	if err != nil {
		return err
	}
	if activityNote != nil {
		notes = append(notes, seedStudentNoteEntry{0, activityNote})
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

// seedStudentNoteEntry is one note the seeder writes about a demo child.
type seedStudentNoteEntry struct {
	studentIndex int
	note         map[string]any
}

// baseSeedStudentNotes are the notes that need no group or activity.
func baseSeedStudentNotes(today seedDate) []seedStudentNoteEntry {
	return []seedStudentNoteEntry{
		{0, map[string]any{
			"kind":       "permanent",
			"visibility": "all_staff",
			"category":   "general",
			"body":       "Braucht morgens etwas Zeit zum Ankommen. Ein ruhiger Platz hilft.",
			// Released for the Lehrkraft of the class (#4797).
			"shared_with_school": true,
		}},
		{0, map[string]any{
			"kind":               "journal",
			"visibility":         "all_staff",
			"category":           "positive",
			"body":               "Hat heute in der Hausaufgabenzeit einem anderen Kind geholfen.",
			"subject_date":       today.String(),
			"shared_with_school": true,
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
}

// seedActivityNote writes an entry about index 0 in one of its activities
// (#4268). The activity comes from the note form's own reference read, so the
// seeder proves the picker and the create route agree on what may be chosen.
// Index 0 is among the first children the master data enrolls in every
// activity.
func seedActivityNote(rt *Runtime, day string) (map[string]any, error) {
	studentID, ok := rt.FixedSeeder.studentIDByIndex[0]
	if !ok {
		return nil, fmt.Errorf("student note demo index 0 not available")
	}
	raw, err := rt.Client.Get(fmt.Sprintf("/api/students/%d/note-subjects", studentID))
	if err != nil {
		return nil, fmt.Errorf("read note subjects: %w", err)
	}
	var response struct {
		Data struct {
			Activities []struct {
				ID string `json:"id"`
			} `json:"activities"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil {
		return nil, fmt.Errorf("parse note subjects: %w", err)
	}
	if len(response.Data.Activities) == 0 {
		fmt.Println("  no activity for an activity note, skipped")
		return nil, nil
	}
	return map[string]any{
		"kind":              "journal",
		"visibility":        "group_leads",
		"category":          "behaviour",
		"body":              "Hat beim Aufräumen nach der AG gut mitgeholfen. Bitte weiter so bestärken.",
		"subject_date":      day,
		"activity_group_id": response.Data.Activities[0].ID,
	}, nil
}
