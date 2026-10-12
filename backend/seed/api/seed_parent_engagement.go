package api

import (
	"context"
	"fmt"
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
	if err := seedParentNotificationPreference(rt, auth); err != nil {
		return err
	}
	if err := seedParentConversation(rt, auth, studentID); err != nil {
		return err
	}
	if err := seedParentGuardianChanges(rt, auth, studentID); err != nil {
		return err
	}
	if err := seedParentPhotoConsentHistory(rt, auth, studentID); err != nil {
		return err
	}
	if err := seedParentMasterData(rt, auth, studentID); err != nil {
		return err
	}
	if err := seedParentMealParticipation(rt, auth, studentID); err != nil {
		return err
	}
	if err := seedParentAppointments(rt, studentID); err != nil {
		return err
	}
	rt.Client.BindAuth(rt.TenantAuth)
	fmt.Println("  1 parent preference, 1 conversation, audited contact/consent changes, 1 master-data request, lunch participation and parent appointments created")
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

// seedParentAppointments fills the families' calendar for the coming weeks
// (#3923). It showed nothing before: only the marketing profile created a
// parent appointment.
func seedParentAppointments(rt *Runtime, studentID int64) error {
	today := todaySeedDate()
	groupID, err := seedStudentGroupID(rt, studentID)
	if err != nil {
		return err
	}
	parentsEvening := seedWeekdayOnOrAfter(today.AddDays(6)).String()
	outing := seedOutingDate(today).String()
	closingDay := seedWeekdayOnOrAfter(today.AddDays(24)).String()
	festival := nextWeekday(today.AddDays(30).UTCMidnight(), time.Friday).Format(seedDateLayout)
	allParents := []map[string]any{{"type": "all_school_parents"}, {"type": "all_staff"}}
	appointments := []map[string]any{
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
		{
			"title":       "Pädagogischer Tag: OGS geschlossen",
			"description": "Das Team bildet sich fort. An diesem Tag findet keine Betreuung statt.",
			"start_date":  closingDay, "end_date": closingDay, "start_time": "00:00", "end_time": "23:59", "all_day": true,
			"delivery_mode": "informational", "targets": allParents, "send_email": false,
		},
		{
			"title": "Laternenfest", "location": "Schulhof",
			"description": "Die Kinder basteln ihre Laternen in der OGS. Wir ziehen gemeinsam durch das Viertel.",
			"start_date":  festival, "end_date": festival, "start_time": "17:00", "end_time": "19:00", "all_day": false,
			"delivery_mode": "rsvp_required", "overview_visibility": "all", "targets": allParents, "send_email": false,
		},
	}
	for _, appointment := range appointments {
		if _, err := rt.Client.PostWithAuth(rt.TenantAuth, "/api/calendar/appointments", appointment); err != nil {
			return fmt.Errorf("create parent appointment %s: %w", appointment["title"], err)
		}
	}
	return nil
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

func seedParentNotificationPreference(rt *Runtime, auth AuthRef) error {
	if _, err := rt.Client.PutWithAuth(auth, "/parent/me/notification-preferences/parent_message", map[string]any{
		"enabled": true,
	}); err != nil {
		return fmt.Errorf("seed parent notification preference: %w", err)
	}
	return nil
}

func seedParentConversation(rt *Runtime, auth AuthRef, studentID int64) error {
	path := fmt.Sprintf("/parent/me/messages/children/%d", studentID)
	raw, err := rt.Client.PostWithAuth(auth, path, map[string]any{
		"body": "Können Sie bitte prüfen, ob die neue Abholzeit eingetragen ist?",
	})
	if err != nil {
		return fmt.Errorf("seed parent message: %w", err)
	}
	threadID, messageID, err := parseThreadMessageIDs(raw)
	if err != nil {
		return fmt.Errorf("parse parent conversation: %w", err)
	}
	if _, err := rt.Client.PostWithAuth(rt.TenantAuth, fmt.Sprintf("/api/messages/threads/%d", threadID), map[string]any{
		"body": "Die neue Abholzeit ist eingetragen.", "handled_up_to_message_id": strconv.FormatInt(messageID, 10),
	}); err != nil {
		return fmt.Errorf("seed staff reply: %w", err)
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

func seedParentGuardianChanges(rt *Runtime, auth AuthRef, studentID int64) error {
	contactRaw, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/parent/me/children/%d/guardians", studentID), map[string]any{
		"first_name": "Rita", "last_name": "Abholkontakt", "email": "rita.abholkontakt@example.test",
		"address_street": "Nebenweg 3", "address_postal_code": "50667", "address_city": "Köln",
		"phones":            []map[string]any{{"phone_number": "+49 221 555 778", "phone_type": "mobile", "label": "Privat", "is_primary": true}},
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
			"email": "rita.abholkontakt@example.test", "confirm_role_upgrade": true,
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
	if _, err := rt.Client.PostWithAuth(auth,
		fmt.Sprintf("/parent/me/children/%d/master-data/requests", studentID),
		map[string]any{
			"changes": []map[string]any{
				{"target": "person", "field_key": "first_name", "value": "Felix-Max"},
			},
			"recipient_guardian_profile_ids": []string{},
		},
	); err != nil {
		return fmt.Errorf("seed parent master-data request: %w", err)
	}
	return nil
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
