package api

import (
	"context"
	"fmt"
)

// seedStaffNoticesStep creates the notices of a school day on the
// Tagesinformationen page: today's ones end today, the weekly one runs until
// the Herbstferien, and the colleagues confirm the notices that ask them to
// (#2208). The author's own confirmation is stamped on creation and is not
// counted, so the confirmations come from other staff accounts.
type seedStaffNoticesStep struct{}

func (seedStaffNoticesStep) Name() string { return "Seeding staff notices" }

// staffNoticeReaders are the staff members, as indices into DemoStaff, who
// confirm the notices asking for it: present today, unlike the colleagues of
// seedTodaysAbsencesStep.
var staffNoticeReaders = []int{0, 1, 3, 4, 11}

var staffNoticeWeekdays = [...]string{"sonntags", "montags", "dienstags", "mittwochs", "donnerstags", "freitags", "samstags"}

// staffNotices are the notices of the day. Zielgruppen (#2208): the
// Räumungsübung and the Notfallplan reach the whole school, the gym only the
// Betreuung and the handover only the Lehrkräfte, so every portal sees
// exactly its share.
func staffNotices(today seedDate) []map[string]any {
	ferienStart, _ := seedHerbstferien(today)
	todayOnly := []int16{int16((int(today.Weekday())+6)%7 + 1)} // ISO weekday, Monday 1
	notice := func(title, body, priority, audience string, until seedDate, acknowledge bool) map[string]any {
		// No weekdays means every day; a notice of one day carries its
		// weekday, otherwise the list reads „täglich · ab 20.10. bis 20.10.“.
		weekdays := []int16{}
		if until.Equal(today.Time) {
			weekdays = todayOnly
		}
		return map[string]any{
			"title": title, "body": body, "priority": priority, "audience": audience,
			"valid_from": today.String(), "valid_until": until.String(),
			"weekdays": weekdays, "week_pattern": 0,
			"requires_acknowledgement": acknowledge, "active": true,
		}
	}
	gym := notice("Turnhalle belegt",
		fmt.Sprintf("Die Turnhalle ist %s bis 15 Uhr durch den Schulsport belegt. Bewegungsangebote finden bis dahin im Bewegungsraum statt.", staffNoticeWeekdays[today.Weekday()]),
		"info", "staff", ferienStart.AddDays(-3), false)
	gym["weekdays"] = todayOnly
	return []map[string]any{
		notice("Räumungsübung um 14:30 Uhr",
			"Heute um 14:30 Uhr ertönt der Probealarm. Bitte begleiten Sie Ihre Gruppe über den Fluchtweg zum Sammelplatz auf dem Schulhof und zählen Sie die Kinder dort.",
			"important", "all", today, true),
		notice("Neuer Notfallplan",
			"Der aktualisierte Notfallplan hängt im OGS-Büro und in jedem Gruppenraum. Bitte lesen Sie ihn in den nächsten Tagen und bestätigen Sie es hier.",
			"important", "all", today.AddDays(6), true),
		gym,
		notice("Übergabe nach der 6. Stunde",
			"Bitte geben Sie die Kinder der 2a heute erst um 13:30 Uhr an die OGS ab. Vorher findet die Klassenstunde statt.",
			"info", "lehrkraft", today, true),
		notice("Neues Bastelmaterial im Werkraum",
			"Im Werkraum liegen neues Transparentpapier, Kleister und Wolle. Bitte tragen Sie Entnahmen in die Liste an der Tür ein.",
			"info", "staff", today.AddDays(13), false),
	}
}

func (seedStaffNoticesStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil {
		return fmt.Errorf("staff notices prerequisites not available")
	}
	readers, err := staffNoticeReaderAuths(rt)
	if err != nil {
		return err
	}
	defer rt.Client.BindAuth(rt.TenantAuth)
	created, confirmations := 0, 0
	for _, notice := range staffNotices(todaySeedDate()) {
		rt.Client.BindAuth(rt.TenantAuth)
		raw, err := rt.Client.Post("/api/staff-notices/", notice)
		if err != nil {
			return fmt.Errorf("create staff notice %q: %w", notice["title"], err)
		}
		created++
		if notice["audience"] != "all" || notice["requires_acknowledgement"] != true {
			continue
		}
		if err := confirmStaffNotice(rt, raw, readers); err != nil {
			return err
		}
		confirmations += len(readers)
	}
	fmt.Printf("  %d staff notices created, %d confirmations\n", created, confirmations)
	return nil
}

// confirmStaffNotice has every reader confirm the notice the response
// created.
func confirmStaffNotice(rt *Runtime, created []byte, readers []AuthRef) error {
	id, err := parseEnvelopeStringID(created)
	if err != nil {
		return fmt.Errorf("parse staff notice response: %w", err)
	}
	for _, reader := range readers {
		if _, err := rt.Client.PostWithAuth(reader, fmt.Sprintf("/api/staff-notices/%d/acknowledge", id), nil); err != nil {
			return fmt.Errorf("acknowledge staff notice: %w", err)
		}
	}
	return nil
}

// staffNoticeReaderAuths signs in the readers the fixed seeder created; a
// school without them (unit fixtures) has fewer confirmations.
func staffNoticeReaderAuths(rt *Runtime) ([]AuthRef, error) {
	if rt.FixedSeeder == nil {
		return nil, nil
	}
	var auths []AuthRef
	for _, index := range staffNoticeReaders {
		cred, ok := staffCredentialsByIndex(rt.FixedSeeder, index)
		if !ok {
			continue
		}
		if err := rt.Client.Login(cred.Email, cred.Password); err != nil {
			return nil, fmt.Errorf("login as staff notice reader %s: %w", cred.Email, err)
		}
		auths = append(auths, rt.Client.auth)
	}
	return auths, nil
}

// staffCredentialsByIndex returns the login of DemoStaff[index].
func staffCredentialsByIndex(fs *FixedSeeder, index int) (StaffCredentials, bool) {
	if index < 0 || index >= len(DemoStaff) {
		return StaffCredentials{}, false
	}
	name := DemoStaff[index].FirstName + " " + DemoStaff[index].LastName
	for _, cred := range fs.staffCredentials {
		if cred.Name == name {
			return cred, true
		}
	}
	return StaffCredentials{}, false
}
