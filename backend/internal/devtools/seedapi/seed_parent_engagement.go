package api

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"
)

type seedParentEngagementStep struct{}

func (seedParentEngagementStep) Name() string { return "Seeding parent engagement" }

func (seedParentEngagementStep) Run(ctx context.Context, rt *Runtime) error {
	if rt == nil || rt.Adapter == nil || rt.Client == nil || len(rt.Parents) == 0 || len(rt.Parents[0].StudentIDs) == 0 {
		return fmt.Errorf("parent engagement prerequisites not available")
	}
	parent := rt.Parents[0]
	auth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
	if err != nil {
		return fmt.Errorf("login parent for engagement demo: %w", err)
	}
	studentID := parent.StudentIDs[0]
	steps := []func() error{
		func() error { return seedParentNotificationPreferences(rt, auth) },
		func() error { return seedParentGroupThreads(rt, auth, studentID) },
		func() error { return seedParentGuardianChanges(rt, auth, studentID) },
		func() error { return seedParentPhotoConsentHistory(rt, auth, studentID) },
		func() error { return seedParentMasterData(rt, auth, studentID) },
		func() error { return seedParentMealParticipation(rt, auth, studentID) },
		// After every step that leaves a note in the chat, so the parents
		// portal opens on a conversation that ends with the OGS's answer.
		func() error { return seedParentConversation(rt, auth, studentID) },
		func() error { return seedParentAppointments(rt, studentID) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	rt.Client.BindAuth(rt.TenantAuth)
	fmt.Println("  parent notification topics, 1 conversation, 2 group threads, audited contact/consent changes, 1 master-data request, lunch participation and parent appointments created")
	return nil
}

// seedParentMealParticipation registers the child for lunch Monday to
// Thursday and changes one day, so the week the parents portal opens with
// shows days with and without lunch (#3923). Regular days only apply from
// the first day still open for changes; the portal opens that day's week.
func seedParentMealParticipation(rt *Runtime, auth AuthRef, studentID int64) error {
	basePath := fmt.Sprintf("/parent/me/children/%d/meal-participation", studentID)
	raw, err := rt.Client.PutWithAuth(auth, basePath, map[string]any{
		"weekdays": []int{1, 2, 3, 4},
	})
	if err != nil {
		return fmt.Errorf("seed regular meal participation: %w", err)
	}
	var resp struct {
		Data struct {
			EffectiveFrom string `json:"effective_from"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &resp); err != nil {
		return fmt.Errorf("parse regular meal participation: %w", err)
	}
	effectiveFrom, err := parseSeedDate(resp.Data.EffectiveFrom)
	if err != nil {
		return fmt.Errorf("parse meal participation start %q: %w", resp.Data.EffectiveFrom, err)
	}
	day, participating := mealParticipationException(effectiveFrom)
	if _, err := rt.Client.PutWithAuth(auth, basePath+"/"+day.String(), map[string]any{
		"participating": participating,
	}); err != nil {
		return fmt.Errorf("seed meal participation exception: %w", err)
	}
	return nil
}

// mealParticipationException picks the changed day in the week of the first
// changeable day: Wednesday off while Wednesday is still ahead, otherwise
// Friday on. Either way the week mixes days with and without lunch.
func mealParticipationException(firstChangeable seedDate) (seedDate, bool) {
	if firstChangeable.Weekday() <= time.Wednesday {
		return firstChangeable.AddDays(int(time.Wednesday - firstChangeable.Weekday())), false
	}
	return firstChangeable.AddDays(int(time.Friday - firstChangeable.Weekday())), true
}

// seedOutingDate is the day of the group's outing: a school day in a little
// over two weeks, after every other dated parent request of the seed.
func seedOutingDate(today seedDate) seedDate {
	return seedWeekdayOnOrAfter(today.AddDays(15))
}

// seedParentsEveningDate is the day of the group's parents' evening, which
// the calendar and the invitation notice share.
func seedParentsEveningDate(today seedDate) seedDate {
	return seedWeekdayOnOrAfter(today.AddDays(6))
}

// seedLanternFestivalDate is the Friday of the Laternenfest, about a month
// out, which the calendar and the Elternbrief share.
func seedLanternFestivalDate(today seedDate) seedDate {
	return seedDate{Time: nextWeekday(today.AddDays(30).UTCMidnight(), time.Friday)}
}

// seedParentAppointments fills the families' calendar for the coming weeks
// (#3923): the group's own dates and the school's.
func seedParentAppointments(rt *Runtime, studentID int64) error {
	today := todaySeedDate()
	groupID, err := seedStudentGroupID(rt, studentID)
	if err != nil {
		return err
	}
	appointments := append(seedGroupAppointments(today, groupID), seedSchoolAppointments(today)...)
	for _, appointment := range appointments {
		if _, err := rt.Client.PostWithAuth(rt.TenantAuth, "/api/calendar/appointments", appointment); err != nil {
			return fmt.Errorf("create parent appointment %s: %w", appointment["title"], err)
		}
	}
	return nil
}

// seedGroupAppointments are the dates of the child's group.
func seedGroupAppointments(today seedDate, groupID int64) []map[string]any {
	parentsEvening := seedParentsEveningDate(today).String()
	outing := seedOutingDate(today).String()
	allParents := []map[string]any{{"type": "all_school_parents"}, {"type": "all_staff"}}
	return []map[string]any{
		{
			"title": "Elternabend der Gruppe", "location": "Gruppenraum",
			"description": "Wir stellen den Tagesablauf vor und planen die Wochen bis zu den Ferien.",
			"start_date":  parentsEvening, "end_date": parentsEvening, "start_time": "19:00", "end_time": "20:30", "all_day": false,
			"delivery_mode": "rsvp_required", "overview_visibility": "all",
			"targets":    []map[string]any{{"type": "parents_by_group", "id": groupID}, {"type": "all_staff"}},
			"send_email": false,
		},
		{
			"title": "Ausflug in den Zoo", "location": "Treffpunkt Schulhof",
			"description": "Bitte geben Sie Ihrem Kind einen Rucksack mit Trinkflasche und Regenjacke mit. Wir sind gegen 15 Uhr zurück.",
			"start_date":  outing, "end_date": outing, "start_time": "08:30", "end_time": "15:00", "all_day": false,
			"delivery_mode": "informational", "targets": allParents, "send_email": false,
		},
	}
}

// seedSchoolAppointments are the dates of the whole school.
func seedSchoolAppointments(today seedDate) []map[string]any {
	photoDay := seedWeekdayOnOrAfter(today.AddDays(9)).String()
	parentCafe := seedWeekdayOnOrAfter(today.AddDays(12)).String()
	holidayDeadline := seedWeekdayOnOrAfter(today.AddDays(21)).String()
	closingDay := seedWeekdayOnOrAfter(today.AddDays(24)).String()
	festival := seedLanternFestivalDate(today).String()
	allParents := []map[string]any{{"type": "all_school_parents"}, {"type": "all_staff"}}
	// Termine, die auch die Schule betreffen, sehen die Lehrkräfte in moto
	// schule (#4798).
	withTeachers := []map[string]any{{"type": "all_school_parents"}, {"type": "all_staff"}, {"type": "all_teachers"}}
	return []map[string]any{
		{
			"title": "Der Schulfotograf kommt", "location": "Aula",
			"description": "Der Fotograf macht Einzel- und Geschwisterfotos. Ob Sie Fotos bestellen, entscheiden Sie danach selbst.",
			"start_date":  photoDay, "end_date": photoDay, "start_time": "08:30", "end_time": "12:30", "all_day": false,
			"delivery_mode": "informational", "targets": withTeachers, "send_email": false,
		},
		{
			"title": "Elterncafé in der OGS", "location": "Mensa",
			"description": "Bei Kaffee und Kuchen lernen Sie das Team und andere Eltern kennen. Die Kinder spielen in der Zeit draußen.",
			"start_date":  parentCafe, "end_date": parentCafe, "start_time": "15:00", "end_time": "16:30", "all_day": false,
			"delivery_mode": "informational", "targets": allParents, "send_email": false,
		},
		{
			"title":       "Anmeldeschluss für die Ferienbetreuung",
			"description": "Bitte melden Sie Ihr Kind bis heute im OGS-Büro für die Betreuung in den nächsten Ferien an.",
			"start_date":  holidayDeadline, "end_date": holidayDeadline, "start_time": "00:00", "end_time": "23:59", "all_day": true,
			"delivery_mode": "informational", "targets": allParents, "send_email": false,
		},
		{
			"title":       "Pädagogischer Tag: OGS geschlossen",
			"description": "Das Team bildet sich fort. An diesem Tag findet keine Betreuung statt.",
			"start_date":  closingDay, "end_date": closingDay, "start_time": "00:00", "end_time": "23:59", "all_day": true,
			"delivery_mode": "informational", "targets": withTeachers, "send_email": false,
		},
		{
			"title": "Laternenfest", "location": "Schulhof",
			"description": "Die Kinder basteln ihre Laternen in der OGS. Wir ziehen gemeinsam durch das Viertel.",
			"start_date":  festival, "end_date": festival, "start_time": "17:00", "end_time": "19:00", "all_day": false,
			"delivery_mode": "rsvp_required", "overview_visibility": "all", "targets": withTeachers, "send_email": false,
		},
	}
}

// seedStudentGroupID reads the group a child belongs to.
func seedStudentGroupID(rt *Runtime, studentID int64) (int64, error) {
	raw, err := rt.Client.GetWithAuth(rt.TenantAuth, fmt.Sprintf("/api/students/%d", studentID))
	if err != nil {
		return 0, fmt.Errorf("load student %d: %w", studentID, err)
	}
	var resp struct {
		Data struct {
			GroupID any `json:"group_id"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &resp); err != nil {
		return 0, fmt.Errorf("parse student %d: %w", studentID, err)
	}
	groupID, err := parseSeedID(resp.Data.GroupID)
	if err != nil || groupID == 0 {
		return 0, fmt.Errorf("student %d has no group", studentID)
	}
	return groupID, nil
}

func seedParentPhotoConsentHistory(rt *Runtime, auth AuthRef, studentID int64) error {
	if _, err := rt.Client.PutWithAuth(rt.TenantAuth, fmt.Sprintf("/api/students/%d", studentID), map[string]any{
		"photo_consent_given": true,
	}); err != nil {
		return fmt.Errorf("seed granted photo consent: %w", err)
	}
	if _, err := rt.Client.DeleteWithAuth(auth, fmt.Sprintf("/parent/me/children/%d/consents/photo", studentID)); err != nil {
		return fmt.Errorf("seed withdrawn parent photo consent: %w", err)
	}
	return nil
}

// seedParentNotificationTopics are the topics the first parent turned on:
// the ones about their child, the school's mail and appointments. The
// appointment reminder stays off, so the settings show a real choice.
var seedParentNotificationTopics = []string{
	"parent_message", "parent_announcement", "parent_appointment", "parent_request_decided", "parent_care_cancelled",
}

func seedParentNotificationPreferences(rt *Runtime, auth AuthRef) error {
	for _, topic := range seedParentNotificationTopics {
		if _, err := rt.Client.PutWithAuth(auth, "/parent/me/notification-preferences/"+topic, map[string]any{
			"enabled": true,
		}); err != nil {
			return fmt.Errorf("seed parent notification preference %s: %w", topic, err)
		}
	}
	return nil
}

// seedParentConversationTurns is the first parent's chat with the OGS: each
// parent message gets its answer, the last one from the OGS. The OGS writes
// without the parent's name, which a demo visitor replaces.
var seedParentConversationTurns = []struct{ parent, staff string }{
	{"Guten Morgen! Felix hat seit dieser Woche eine Brille. Darf er sie beim Fußball absetzen?", "Guten Morgen! Ja, natürlich. Er legt sie dann in sein Fach."},
	{"Danke! Können Sie bitte noch prüfen, ob die neue Abholzeit eingetragen ist?", "Die neue Abholzeit ist eingetragen."},
	{"Prima. Felix freut sich übrigens schon sehr auf den Zoo.", "Das hören wir gern. Die Kinder sprechen schon die ganze Woche davon."},
}

func seedParentConversation(rt *Runtime, auth AuthRef, studentID int64) error {
	path := fmt.Sprintf("/parent/me/messages/children/%d", studentID)
	var threadID int64
	for _, turn := range seedParentConversationTurns {
		raw, err := rt.Client.PostWithAuth(auth, path, map[string]any{"body": turn.parent})
		if err != nil {
			return fmt.Errorf("seed parent message: %w", err)
		}
		var messageID int64
		threadID, messageID, err = parseThreadMessageIDs(raw)
		if err != nil {
			return fmt.Errorf("parse parent conversation: %w", err)
		}
		if _, err := rt.Client.PostWithAuth(rt.TenantAuth, fmt.Sprintf("/api/messages/threads/%d", threadID), map[string]any{
			"body": turn.staff, "handled_up_to_message_id": strconv.FormatInt(messageID, 10),
		}); err != nil {
			return fmt.Errorf("seed staff reply: %w", err)
		}
	}
	// The team keeps the answered conversation open for a colleague (#3654),
	// so the inbox shows a thread marked unread for everyone.
	if _, err := rt.Client.PostWithAuth(rt.TenantAuth, fmt.Sprintf("/api/messages/threads/%d/unread", threadID), map[string]any{}); err != nil {
		return fmt.Errorf("seed conversation marked unread: %w", err)
	}
	if _, err := rt.Client.GetWithAuth(auth, path); err != nil {
		return fmt.Errorf("read seeded parent conversation: %w", err)
	}
	return nil
}

// seedPickupContactEmail is the address of the pickup contact the first
// parent adds. The reserved .example domain reaches nobody.
const seedPickupContactEmail = "gisela.hahn@mail.example"

func seedParentGuardianChanges(rt *Runtime, auth AuthRef, studentID int64) error {
	contactRaw, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/parent/me/children/%d/guardians", studentID), map[string]any{
		"first_name": "Gisela", "last_name": "Hahn", "email": seedPickupContactEmail,
		"address_street": "Nebenweg 3", "address_postal_code": "50667", "address_city": "Köln",
		"phones":            []map[string]any{{"phone_number": "+49 172 5550778", "phone_type": "mobile", "is_primary": true}},
		"relationship_type": "relative", "can_pickup": true, "is_emergency_contact": false,
	})
	if err != nil {
		return fmt.Errorf("seed parent-managed pickup contact: %w", err)
	}
	contactID, err := parseGuardianProfileID(contactRaw)
	if err != nil {
		return fmt.Errorf("parse parent-managed pickup contact: %w", err)
	}
	if _, err := rt.Client.PutWithAuth(auth,
		fmt.Sprintf("/parent/me/children/%d/guardians/%d/pickup", studentID, contactID),
		map[string]any{"can_pickup": false, "is_emergency_contact": true, "pickup_notes": "Nur nach telefonischer Rücksprache"},
	); err != nil {
		return fmt.Errorf("update parent-managed pickup contact: %w", err)
	}
	return withTemporarySeedSetting(rt, rt.TenantAuth, "guardians.parent_invite_mode", "staff_approval", "direct", func() error {
		if _, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/parent/me/children/%d/related-accounts", studentID), map[string]any{
			"email": seedPickupContactEmail, "confirm_role_upgrade": true,
		}); err != nil {
			return fmt.Errorf("seed pending guardian role upgrade: %w", err)
		}
		return nil
	})
}

func seedParentMasterData(rt *Runtime, auth AuthRef, studentID int64) error {
	if _, err := rt.Client.PatchWithAuth(auth,
		fmt.Sprintf("/parent/me/children/%d/master-data/guardian_profile/preferred_contact_method", studentID),
		map[string]any{"value": "phone"},
	); err != nil {
		return fmt.Errorf("seed parent contact preference: %w", err)
	}
	modes, err := seedFridayDepartureModes(rt, auth, studentID)
	if err != nil {
		return err
	}
	if _, err := rt.Client.PostWithAuth(auth,
		fmt.Sprintf("/parent/me/children/%d/master-data/requests", studentID),
		map[string]any{
			"changes": []map[string]any{
				{"target": "departure", "field_key": "allowed_departure_modes", "value": modes},
			},
			"recipient_guardian_profile_ids": []string{},
		},
	); err != nil {
		return fmt.Errorf("seed parent master-data request: %w", err)
	}
	return nil
}

// seedFridayDepartureModes reads the child's permanent ways home and asks
// for one clear change the staff can decide at a glance: on Fridays the
// child walks home alone from now on.
func seedFridayDepartureModes(rt *Runtime, auth AuthRef, studentID int64) (map[string][]string, error) {
	raw, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/parent/me/children/%d/master-data", studentID))
	if err != nil {
		return nil, fmt.Errorf("load parent master data: %w", err)
	}
	var resp struct {
		Data struct {
			AllowedDepartureModes map[string][]string `json:"allowed_departure_modes"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse parent master data: %w", err)
	}
	return fridayDepartureChange(resp.Data.AllowedDepartureModes), nil
}

// fridayDepartureChange changes only Friday of the current ways home: to
// walking alone, or, for a child who walks alone on Fridays already (a day
// without modes means alone), to being picked up.
func fridayDepartureChange(current map[string][]string) map[string][]string {
	modes := make(map[string][]string, len(current)+1)
	for day, list := range current {
		modes[day] = slices.Clone(list)
	}
	friday := modes["fri"]
	if len(friday) == 0 || slices.Equal(friday, []string{"alone"}) {
		modes["fri"] = []string{"pickup"}
		return modes
	}
	modes["fri"] = []string{"alone"}
	return modes
}

func parseGuardianProfileID(raw []byte) (int64, error) {
	var envelope struct {
		Data struct {
			GuardianProfileID string `json:"guardian_profile_id"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &envelope); err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(envelope.Data.GuardianProfileID, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid guardian_profile_id %q", envelope.Data.GuardianProfileID)
	}
	return id, nil
}

func parseThreadMessageIDs(raw []byte) (int64, int64, error) {
	var envelope struct {
		Data struct {
			ThreadID string `json:"thread_id"`
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &envelope); err != nil {
		return 0, 0, err
	}
	id, err := strconv.ParseInt(envelope.Data.ThreadID, 10, 64)
	if err != nil || id <= 0 {
		return 0, 0, fmt.Errorf("invalid thread_id %q", envelope.Data.ThreadID)
	}
	if len(envelope.Data.Messages) == 0 {
		return 0, 0, fmt.Errorf("parent conversation has no messages")
	}
	messageID, err := strconv.ParseInt(envelope.Data.Messages[len(envelope.Data.Messages)-1].ID, 10, 64)
	if err != nil || messageID <= 0 {
		return 0, 0, fmt.Errorf("invalid message id")
	}
	return id, messageID, nil
}
