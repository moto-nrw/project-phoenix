package api

import (
	"context"
	"fmt"
	"strconv"
)

// seedSchoolLifeStep fills the everyday surfaces of the demo school a little
// more, the way a running school fills them: team appointments in the current
// week, two published notices for the families, and messages from two more
// families. The demo and the product screenshots show these pages.
type seedSchoolLifeStep struct{}

func (seedSchoolLifeStep) Name() string { return "Seeding everyday school life" }

func (seedSchoolLifeStep) Run(ctx context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	if err := seedTeamWeekAppointments(rt); err != nil {
		return err
	}
	if err := seedEverydayNotices(rt); err != nil {
		return err
	}
	if len(rt.Parents) == 0 {
		return fmt.Errorf("everyday school life needs the parent accounts")
	}
	conversations := []struct {
		parent      ParentCredentials
		message     string
		staffAnswer string
	}{
		{requestParent(rt.Parents, 2), "Hat jemand eine blaue Trinkflasche gefunden? Sie ist seit gestern weg.", "Ja, sie stand in der Mensa. Wir haben sie in den Rucksack gesteckt."},
		{requestParent(rt.Parents, 3), "Kann mein Kind morgen mit zum Fußball? Die Sportsachen sind eingepackt.", ""},
	}
	for _, c := range conversations {
		if err := seedFamilyConversation(ctx, rt, c.parent, c.message, c.staffAnswer); err != nil {
			return err
		}
	}
	fmt.Println("  3 team appointments, 2 notices and 2 family conversations created")
	return nil
}

// seedTeamWeekAppointments puts three team appointments into the current
// Monday-to-Friday week, so the staff calendar of any weekday shows a week
// with more than the weekly team meeting.
func seedTeamWeekAppointments(rt *Runtime) error {
	today := todaySeedDate()
	monday := today.AddDays(-((int(today.Weekday()) + 6) % 7))
	staff := []map[string]any{{"type": "all_staff"}}
	appointments := []struct {
		title, location, description string
		day                          int
		start, end                   string
	}{
		{"Absprache Ferienprogramm", "OGS-Büro", "Ausflüge und Angebote für die Weihnachtsferien festlegen.", 0, "08:30", "09:15"},
		{"Hospitation in der Klasse 2b", "Klasse 2b", "Gemeinsamer Blick auf die Lernzeit mit der Klassenlehrerin.", 1, "10:00", "11:00"},
		{"Elterngespräche Sternengruppe", "Gruppenraum", "Drei Gespräche zur Eingewöhnung, je 20 Minuten.", 3, "15:30", "16:30"},
	}
	for _, a := range appointments {
		day := monday.AddDays(a.day).String()
		if _, err := rt.Client.Post("/api/calendar/appointments", map[string]any{
			"title": a.title, "location": a.location, "description": a.description,
			"start_date": day, "end_date": day, "start_time": a.start, "end_time": a.end, "all_day": false,
			"delivery_mode": "informational", "targets": staff, "send_email": false,
		}); err != nil {
			return fmt.Errorf("create team appointment %s: %w", a.title, err)
		}
	}
	return nil
}

// seedEverydayNotices publishes two notices to all families. No e-mail: the
// demo environment sends real mail, and the seeded addresses are not ours.
func seedEverydayNotices(rt *Runtime) error {
	notices := []struct{ title, body string }{
		{"Wechselkleidung für die kalten Tage", "Liebe Eltern, bitte geben Sie Ihrem Kind ab sofort Matschhose und Gummistiefel mit. Wir sind auch bei Regen draußen."},
		{"Neue Bücher in der Leseecke", "Der Förderverein hat uns zwanzig neue Kinderbücher geschenkt. Die Kinder können sie ab Montag in der Leseecke ausleihen."},
	}
	for _, n := range notices {
		raw, err := rt.Client.Post("/api/parent-announcements/", map[string]any{
			"title": n.title, "body": n.body, "priority": "info", "send_email": false,
			"targets": []map[string]any{{"target_type": "school_all"}},
		})
		if err != nil {
			return fmt.Errorf("create notice %s: %w", n.title, err)
		}
		id, err := parseEnvelopeStringID(raw)
		if err != nil {
			return fmt.Errorf("parse notice %s: %w", n.title, err)
		}
		if _, err := rt.Client.Post(fmt.Sprintf("/api/parent-announcements/%d/publish", id), nil); err != nil {
			return fmt.Errorf("publish notice %s: %w", n.title, err)
		}
	}
	return nil
}

// seedFamilyConversation sends one parent message about the parent's child
// and, with a staff answer, replies as the OGS.
func seedFamilyConversation(ctx context.Context, rt *Runtime, parent ParentCredentials, message, staffAnswer string) error {
	auth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
	if err != nil {
		return fmt.Errorf("login parent %s: %w", parent.Email, err)
	}
	raw, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/parent/me/messages/children/%d", parent.StudentIDs[0]), map[string]any{"body": message})
	if err != nil {
		return fmt.Errorf("send parent message for %s: %w", parent.Email, err)
	}
	if staffAnswer == "" {
		return nil
	}
	threadID, messageID, err := parseThreadMessageIDs(raw)
	if err != nil {
		return fmt.Errorf("parse parent message for %s: %w", parent.Email, err)
	}
	if _, err := rt.Client.PostWithAuth(rt.TenantAuth, fmt.Sprintf("/api/messages/threads/%d", threadID), map[string]any{
		"body": staffAnswer, "handled_up_to_message_id": strconv.FormatInt(messageID, 10),
	}); err != nil {
		return fmt.Errorf("answer parent message for %s: %w", parent.Email, err)
	}
	return nil
}
