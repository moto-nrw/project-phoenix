package api

import (
	"context"
	"fmt"
	"slices"
	"time"
)

// seedVisitorDayStep gives the caregiver whose place the visitor of a public
// demo takes (visitorStaffIndex) a day of her own (#3922). Before, every
// planned block went to the first four staff members, so "Mein Tag", "Heute
// geplant" and "Mein Kalender" stayed empty for the visitor. The local seed
// shows the same person with the same day.
//
// Her day on every weekday: Mittagessen and, Monday, Wednesday and Friday,
// "Lernzeit Jahrgang 1" (both joined where they are created), plus the blocks
// below. Her calendar gets appointments in the week it opens on, and a
// colleague has a birthday on the seed day.
type seedVisitorDayStep struct{}

func (seedVisitorDayStep) Name() string { return "Seeding the visitor's own day" }

// The colleague with a birthday on the seed day. Not the visitor: her own
// birthday is the prospect's private data.
const birthdayColleagueIndex = 12

func (seedVisitorDayStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.FixedSeeder == nil {
		return fmt.Errorf("visitor day prerequisites not available")
	}
	staffID := visitorStaffID(rt.FixedSeeder)
	if staffID == 0 {
		return fmt.Errorf("visitor day needs the visitor's staff member")
	}
	rt.Client.BindAuth(rt.TenantAuth)
	today := todaySeedDate()
	if err := seedVisitorBlocks(rt, today, staffID); err != nil {
		return err
	}
	if err := seedVisitorAppointments(rt, today, staffID); err != nil {
		return err
	}
	if err := seedColleagueBirthday(rt, today); err != nil {
		return err
	}
	fmt.Println("  4 blocks, 3 appointments and 1 colleague birthday for the visitor's day")
	return nil
}

// visitorStaffID is the staff member the visitor of a demo school becomes;
// zero when the seed has not created that person.
func visitorStaffID(fs *FixedSeeder) int64 {
	staff := DemoStaff[visitorStaffIndex]
	return fs.staffIDs[staff.FirstName+" "+staff.LastName]
}

// withVisitorStaff adds the visitor's staff member to a block's staff, so a
// block the school already plans shows up in the visitor's day as well.
func withVisitorStaff(fs *FixedSeeder, staffIDs []int64) []int64 {
	out := append([]int64(nil), staffIDs...)
	if id := visitorStaffID(fs); id != 0 && !slices.Contains(out, id) {
		out = append(out, id)
	}
	return out
}

// seedVisitorBlocks plans the blocks only the visitor leads. Children of the
// Sternengruppe, the group she leads, are free at these times; the two AGs take
// children of other groups, who have no other block then.
func seedVisitorBlocks(rt *Runtime, today seedDate, staffID int64) error {
	fs := rt.FixedSeeder
	groupID := fs.groupIDs["sternengruppe"]
	studentIDs := orderedSeedStudentIDs(fs)
	if groupID == 0 || len(studentIDs) < 40 {
		return fmt.Errorf("visitor blocks need the Sternengruppe and forty children")
	}
	group := studentIDs[:10]
	blocks := []map[string]any{
		{
			"name": "Lernzeit Sternengruppe", "type": "care", "list_kind": "learning_time",
			"target_group_type": "gruppe", "education_group_id": groupID,
			"targets":  []map[string]any{{"type": "gruppe", "education_group_id": groupID}},
			"weekdays": []int{2, 4}, "start_time": "13:00", "end_time": "14:00",
			"room": "OGS-Raum 2", "category": "Hausaufgaben", "student_ids": group,
		},
		{
			"name": "Lese-AG", "type": "activity", "list_kind": "activity", "target_group_type": "none",
			"weekdays": []int{1, 3, 5}, "start_time": "14:00", "end_time": "15:00",
			"room": "Leseecke", "category": "Lernen", "student_ids": studentIDs[30:38],
		},
		{
			"name": "Kunst-AG", "type": "activity", "list_kind": "activity", "target_group_type": "none",
			"weekdays": []int{2, 4}, "start_time": "14:00", "end_time": "15:00",
			"room": "Kreativraum", "category": "Kreativ", "student_ids": studentIDs[20:30],
		},
		{
			"name": "Spielen und Abholen", "type": "care",
			"target_group_type": "gruppe", "education_group_id": groupID,
			"targets":  []map[string]any{{"type": "gruppe", "education_group_id": groupID}},
			"weekdays": []int{1, 2, 3, 4, 5}, "start_time": "15:00", "end_time": "16:30",
			"room": "OGS-Raum 2", "category": "Gruppenraum", "student_ids": group,
		},
	}
	for _, body := range blocks {
		roomID := fs.roomIDs[body["room"].(string)]
		categoryID := fs.categoryIDs[body["category"].(string)]
		if categoryID == 0 {
			categoryID = fs.categoryIDs["Gruppenraum"]
		}
		if roomID == 0 || categoryID == 0 {
			return fmt.Errorf("visitor block %s: room or category missing", body["name"])
		}
		delete(body, "room")
		delete(body, "category")
		body["room_id"], body["category_id"], body["week_pattern"] = roomID, categoryID, 0
		body["staff_ids"], body["primary_staff_id"] = []int64{staffID}, staffID
		// Two weeks, like the other demo blocks; the weekly materialization
		// carries them on.
		body["materialize_from"], body["materialize_to"] = today.String(), today.AddDays(13).String()
		if _, err := rt.Client.Post("/api/timetable/templates", body); err != nil {
			return fmt.Errorf("create visitor block %s: %w", body["name"], err)
		}
	}
	return nil
}

// seedVisitorAppointments fills the week "Mein Kalender" opens on: the week of
// the seed day, Monday to Friday. A school seeded on a weekend has no blocks
// and no shifts left in that week, so the appointments carry it. They address
// staff only: the parent accounts do not exist yet at this step.
func seedVisitorAppointments(rt *Runtime, today seedDate, staffID int64) error {
	monday := seedDate{Time: mostRecentWeekday(today.Time, time.Monday)}
	wednesday, thursday := monday.AddDays(2).String(), monday.AddDays(3).String()
	occurrences := 6
	// Mornings, while the children are at school; the afternoons hold blocks.
	appointments := []map[string]any{
		{
			"title": "Elterngespräch", "location": "OGS-Raum 2",
			"description": "Gespräch über die Eingewöhnung in der Sternengruppe.",
			"start_date":  monday.String(), "end_date": monday.String(), "start_time": "10:00", "end_time": "10:30", "all_day": false,
			"delivery_mode": "informational", "targets": []map[string]any{{"type": "staff", "id": staffID}}, "send_email": false,
		},
		{
			"title": "Teambesprechung", "location": "Lehrerzimmer",
			"description": "Wochenrückblick, Absprachen zu den AGs und Termine der nächsten Woche.",
			"start_date":  wednesday, "end_date": wednesday, "start_time": "11:00", "end_time": "11:45", "all_day": false,
			"recurrence": map[string]any{
				"frequency": "weekly", "interval_count": 1, "weekdays": []string{"wednesday"}, "occurrence_count": occurrences,
			},
			"delivery_mode": "informational", "targets": []map[string]any{{"type": "all_staff"}}, "send_email": false,
		},
		{
			"title": "Erste-Hilfe-Auffrischung", "location": "Aula",
			"description": "Pflichtfortbildung für das Team: Erste Hilfe am Kind.",
			"start_date":  thursday, "end_date": thursday, "start_time": "08:30", "end_time": "11:30", "all_day": false,
			"delivery_mode": "informational", "targets": []map[string]any{{"type": "staff", "id": staffID}}, "send_email": false,
		},
	}
	for _, appointment := range appointments {
		if _, err := rt.Client.Post("/api/calendar/appointments", appointment); err != nil {
			return fmt.Errorf("create visitor appointment %s: %w", appointment["title"], err)
		}
	}
	return nil
}

// seedColleagueBirthday gives one colleague a birthday on the seed day; the
// profile shows staff birthdays (operations.birthday_display_include_staff).
// The person form saves the names along with the birthday, so it sends the
// names the seed created the person with.
func seedColleagueBirthday(rt *Runtime, today seedDate) error {
	fs := rt.FixedSeeder
	staff := DemoStaff[birthdayColleagueIndex]
	staffID := fs.staffIDs[staff.FirstName+" "+staff.LastName]
	if staffID == 0 {
		return fmt.Errorf("birthday colleague not available")
	}
	visitor := DemoStaff[visitorStaffIndex]
	first, last := visitorDisplayName(fs.visitor, false, staff.FirstName, staff.LastName, visitor.FirstName, visitor.LastName)
	birthday := fmt.Sprintf("1988-%02d-%02d", int(today.Month()), today.Day())
	if _, err := rt.Client.Put(fmt.Sprintf("/api/staff/%d/stammdaten/person", staffID), map[string]any{
		"first_name": first, "last_name": last, "birthday": birthday, "note": "Geburtsdatum ergänzt",
	}); err != nil {
		return fmt.Errorf("seed colleague birthday: %w", err)
	}
	return nil
}
