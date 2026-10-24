package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// seedAnnouncementsStep creates demo operator announcements.
type seedAnnouncementsStep struct{}

func (seedAnnouncementsStep) Name() string { return "Seeding announcements" }

func (seedAnnouncementsStep) Run(ctx context.Context, rt *Runtime) error {
	// Switch to operator auth for announcements
	rt.Client.BindAuth(rt.OperatorAuth)
	defer rt.Client.BindAuth(rt.TenantAuth) // restore tenant auth

	announcements := []map[string]any{
		{
			"title":    "Willkommen im neuen Schuljahr 2026/27",
			"content":  "Liebe Kolleginnen und Kollegen, wir freuen uns auf ein neues Schuljahr! Bitte überprüft eure Dienstpläne und meldet euch bei Fragen im OGS-Büro.",
			"type":     "announcement",
			"severity": "info",
		},
		{
			"title":    "Wartungsarbeiten am Freitag, 11.04.",
			"content":  "Am Freitag zwischen 18:00 und 20:00 Uhr wird das System für Wartungsarbeiten kurzzeitig nicht verfügbar sein. Bitte plant entsprechend.",
			"type":     "maintenance",
			"severity": "warning",
		},
		{
			"title":    "Neue Funktion: Abholzeiten jetzt im System",
			"content":  "Ab sofort können Abholzeiten pro Kind und Wochentag hinterlegt werden. Die Zeiten werden auf dem RFID-Terminal angezeigt, wenn ein Kind abgeholt wird.",
			"type":     "release",
			"severity": "info",
		},
	}

	var firstAnnouncementID int64
	for _, a := range announcements {
		raw, err := rt.Client.Post("/operator/announcements", a)
		if err != nil {
			return fmt.Errorf("create announcement %q: %w", a["title"], err)
		}
		if firstAnnouncementID == 0 {
			firstAnnouncementID, err = parseEnvelopeStringID(raw)
			if err != nil {
				return fmt.Errorf("parse operator announcement: %w", err)
			}
		}
	}
	if firstAnnouncementID != 0 {
		rt.Client.BindAuth(rt.TenantAuth)
		if _, err := rt.Client.Post(fmt.Sprintf("/api/platform/announcements/%d/seen", firstAnnouncementID), nil); err != nil {
			return fmt.Errorf("mark operator announcement seen: %w", err)
		}
	}

	fmt.Printf("  %d announcements created\n", len(announcements))
	return nil
}

// seedParentLetterStep runs after parentEnrollmentSeedStep so its portal-only
// audience resolves the demo guardians and the delivery matrix is populated.
type seedParentLetterStep struct{}

func (seedParentLetterStep) Name() string { return "Seeding parent letter" }

func (seedParentLetterStep) Run(_ context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	return seedParentLetter(rt)
}

func seedParentLetter(rt *Runtime) error {
	raw, err := rt.Client.Post("/api/parent-announcements/", map[string]any{
		"title":          "Elternbrief: Ausflug am Freitag",
		"body":           "Liebe Eltern, am Freitag fahren wir in den Zoo. Bitte bestätigen Sie den Brief im Elternportal.",
		"priority":       "important",
		"delivery_mode":  "letter",
		"email_audience": "portal_only",
		"targets": []map[string]any{
			{"target_type": "school_all"},
		},
	})
	if err != nil {
		return fmt.Errorf("create parent letter: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse parent letter response: %w", err)
	}
	// Anhang VOR dem Veröffentlichen: danach ist die Mitteilung unveränderlich
	// und das Backend lehnt jeden weiteren Anhang ab (#2890). Ohne diese Zeile
	// ist der Anhangsbereich auf jeder Entwicklungsmaschine leer, und genau der
	// Teil — was die Eltern am Ende herunterladen — lässt sich dann nirgends
	// ansehen.
	if _, err := rt.Client.PostFile(
		fmt.Sprintf("/api/announcement-attachments/%d", id),
		"file",
		"Zoobesuch Einverständnis.pdf",
		demoPDF("Einverständnis Zoobesuch", []string{
			"Mein Kind darf am Freitag mit in den Zoo.",
			"Wir sind um 16:00 Uhr zurück an der Schule.",
			"Bitte geben Sie festes Schuhwerk und eine Regenjacke mit.",
		}),
	); err != nil {
		return fmt.Errorf("attach file to parent letter: %w", err)
	}

	publishedRaw, err := rt.Client.Post(fmt.Sprintf("/api/parent-announcements/%d/publish", id), nil)
	if err != nil {
		return fmt.Errorf("publish parent letter: %w", err)
	}
	publishedAt, err := parseEnvelopePublishedAt(publishedRaw)
	if err != nil {
		return fmt.Errorf("parse published parent letter: %w", err)
	}
	if len(rt.Parents) > 0 && rt.Adapter != nil {
		parent := rt.Parents[0]
		auth, err := rt.Adapter.LoginParent(context.Background(), parent.Email, parent.Password)
		if err != nil {
			return fmt.Errorf("login parent to read parent letter: %w", err)
		}
		if _, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/parent/me/news/%d/read", id), map[string]any{
			"published_at": publishedAt,
		}); err != nil {
			return fmt.Errorf("mark parent letter read: %w", err)
		}
		rt.Client.BindAuth(rt.TenantAuth)
	}
	fmt.Println("  1 parent letter published")
	return seedReminderAnnouncement(rt)
}

// seedParentPollStep creates a full 60-option poll so the staff editor and
// parent portal's long-poll layout have representative demo data (#3861).
type seedParentPollStep struct{}

func (seedParentPollStep) Name() string { return "Seeding parent poll" }

func (seedParentPollStep) Run(_ context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	return seedParentPoll(rt)
}

func seedParentPoll(rt *Runtime) error {
	raw, err := rt.Client.Post("/api/parent-announcements/", map[string]any{
		"title":         "Termine für den Elternsprechtag",
		"body":          "Bitte wählen Sie passende Termine.",
		"priority":      "info",
		"response_type": "multi_choice",
		"options":       seedParentPollOptions(),
		"targets": []map[string]any{
			{"target_type": "school_all"},
		},
	})
	if err != nil {
		return fmt.Errorf("create parent poll: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse parent poll response: %w", err)
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/parent-announcements/%d/publish", id), nil); err != nil {
		return fmt.Errorf("publish parent poll: %w", err)
	}
	fmt.Println("  1 parent poll with 60 appointment options published")
	return nil
}

func seedParentPollOptions() []string {
	days := []struct {
		weekday string
		date    string
	}{
		{weekday: "Mittwoch", date: "14. Oktober"},
		{weekday: "Donnerstag", date: "15. Oktober"},
		{weekday: "Freitag", date: "16. Oktober"},
	}
	options := make([]string, 0, 60)
	for _, day := range days {
		for slot := range 20 {
			hour := 14 + slot/4
			minute := 15 * (slot % 4)
			options = append(options, fmt.Sprintf("%s, %s, %02d:%02d Uhr", day.weekday, day.date, hour, minute))
		}
	}
	return options
}

// seedReminderAnnouncement publishes the announcement the reminder feature
// (#3162) was asked for: written early so families can plan, reminded the
// morning before it matters. Without it the reminder fields are empty on
// every development machine and nobody ever sees the "Erinnerung" state.
func seedReminderAnnouncement(rt *Runtime) error {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return fmt.Errorf("load Berlin location: %w", err)
	}
	now := time.Now().In(berlin)
	// Tomorrow 08:00 Berlin: in the future on every machine, so the create
	// validation accepts it, and the tick delivers it the next morning.
	reminderAt := time.Date(now.Year(), now.Month(), now.Day()+1, 8, 0, 0, 0, berlin)
	raw, err := rt.Client.Post("/api/parent-announcements/", map[string]any{
		"title":         "Betreuung endet am letzten Schultag um 13:00 Uhr",
		"body":          "Liebe Eltern, am letzten Schultag vor den Ferien endet die Betreuung bereits um 13:00 Uhr. Bitte holen Sie Ihr Kind bis dahin ab oder geben Sie uns Bescheid, wenn es allein nach Hause gehen darf.",
		"priority":      "info",
		"send_email":    true,
		"reminder_at":   reminderAt.Format(time.RFC3339),
		"reminder_text": "Morgen endet die Betreuung um 13:00 Uhr. Bitte holen Sie Ihr Kind bis dahin ab.",
		"targets": []map[string]any{
			{"target_type": "school_all"},
		},
	})
	if err != nil {
		return fmt.Errorf("create reminder announcement: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse reminder announcement response: %w", err)
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/parent-announcements/%d/publish", id), nil); err != nil {
		return fmt.Errorf("publish reminder announcement: %w", err)
	}
	fmt.Println("  1 parent announcement with scheduled reminder published")
	return nil
}

// seedParentDeclarationStep runs after the parent letter: the Erklärung
// (#3430) reaches the same demo guardians.
type seedParentDeclarationStep struct{}

func (seedParentDeclarationStep) Name() string { return "Seeding parent declaration" }

func (seedParentDeclarationStep) Run(_ context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	return seedParentDeclaration(rt)
}

// seedDeclarationTitle names the seeded Erklärung; the parent step finds it
// in the feed by it.
const seedDeclarationTitle = "Einverständnis: Ausflug in den Zoo"

// seedParentDeclaration publishes an Erklärung (#3430) with a PDF and lets
// the first demo parent consent for one child, so the staff status, the
// proof and the parent view all have something to show on every machine.
func seedParentDeclaration(rt *Runtime) error {
	deadline := time.Now().AddDate(0, 0, 14)
	raw, err := rt.Client.Post("/api/parent-announcements/", map[string]any{
		"title":                         seedDeclarationTitle,
		"body":                          "Liebe Eltern, am Freitag fahren wir mit dem Bus in den Zoo. Wir sind um 16:00 Uhr zurück an der Schule. Bitte geben Sie an, ob Ihr Kind mitfahren darf.",
		"priority":                      "important",
		"delivery_mode":                 "declaration",
		"declaration_kind":              "consent",
		"declaration_signers":           "any",
		"declaration_revocable":         true,
		"declaration_requires_password": false,
		"response_deadline":             deadline.Format(time.RFC3339),
		"send_email":                    true,
		"targets":                       []map[string]any{{"target_type": "school_all"}},
	})
	if err != nil {
		return fmt.Errorf("create parent declaration: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse parent declaration response: %w", err)
	}
	// The document belongs to the frozen version, so it is attached before
	// the first publication (after that the attachments are fixed).
	if _, err := rt.Client.PostFile(
		fmt.Sprintf("/api/announcement-attachments/%d", id), "file", "Einverständnis Zoo.pdf",
		demoPDF("Einverständnis Ausflug in den Zoo", []string{
			"Abfahrt: Freitag, 8:30 Uhr an der Schule.",
			"Rückkehr: 16:00 Uhr an der Schule.",
			"Bitte geben Sie Ihrem Kind etwas zu trinken mit.",
		}),
	); err != nil {
		return fmt.Errorf("attach file to parent declaration: %w", err)
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/parent-announcements/%d/publish", id), nil); err != nil {
		return fmt.Errorf("publish parent declaration: %w", err)
	}
	if len(rt.Parents) > 0 && rt.Adapter != nil {
		if err := seedDeclarationAnswer(rt); err != nil {
			return err
		}
	}
	fmt.Println("  1 parent declaration published")
	return nil
}

// seedDeclarationAnswer lets the first demo parent consent for their first
// child through the parent API, exactly as the portal does.
func seedDeclarationAnswer(rt *Runtime) error {
	defer rt.Client.BindAuth(rt.TenantAuth)
	parent := rt.Parents[0]
	auth, err := rt.Adapter.LoginParent(context.Background(), parent.Email, parent.Password)
	if err != nil {
		return fmt.Errorf("login parent to declare: %w", err)
	}
	feedRaw, err := rt.Client.GetWithAuth(auth, "/parent/me/news")
	if err != nil {
		return fmt.Errorf("load parent feed for declaration: %w", err)
	}
	var feed struct {
		Data []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Declaration *struct {
				Version struct {
					ID string `json:"id"`
				} `json:"version"`
				Children []struct {
					StudentID string `json:"student_id"`
					CanSubmit bool   `json:"can_submit"`
				} `json:"children"`
			} `json:"declaration"`
		} `json:"data"`
	}
	if err := json.Unmarshal(feedRaw, &feed); err != nil {
		return fmt.Errorf("parse parent feed for declaration: %w", err)
	}
	for _, item := range feed.Data {
		if item.Title != seedDeclarationTitle || item.Declaration == nil {
			continue
		}
		for _, child := range item.Declaration.Children {
			if !child.CanSubmit {
				continue
			}
			if _, err := rt.Client.PostWithAuth(auth, "/parent/me/news/"+item.ID+"/declaration", map[string]any{
				"student_id": child.StudentID, "action": "agreed", "version_id": item.Declaration.Version.ID,
			}); err != nil {
				return fmt.Errorf("declare consent: %w", err)
			}
			return nil
		}
	}
	return errors.New("seeded parent declaration is not answerable by the first demo parent")
}

func parseEnvelopePublishedAt(raw []byte) (time.Time, error) {
	var envelope struct {
		Data struct {
			PublishedAt time.Time `json:"published_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return time.Time{}, err
	}
	if envelope.Data.PublishedAt.IsZero() {
		return time.Time{}, fmt.Errorf("published_at missing")
	}
	return envelope.Data.PublishedAt, nil
}

// seedPrivacyConsentsStep creates privacy consent records for all students.
type seedPrivacyConsentsStep struct{}

func (seedPrivacyConsentsStep) Name() string { return "Seeding privacy consents" }

func (seedPrivacyConsentsStep) Run(ctx context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}

	count := 0
	for i, student := range DemoStudents {
		studentID, ok := rt.FixedSeeder.studentIDByIndex[i]
		if !ok {
			return fmt.Errorf("student ID not available for privacy consent index %d", i)
		}
		_ = student // used only for index

		_, err := rt.Client.Put(fmt.Sprintf("/api/students/%d/privacy-consent", studentID), map[string]any{
			"policy_version":      "1.0",
			"accepted":            true,
			"duration_days":       365,
			"data_retention_days": 30,
			"details": map[string]any{
				"accepted_by": "guardian",
				"accepted_at": time.Now().AddDate(0, -1, 0).Format(time.RFC3339),
			},
		})
		if err != nil {
			return fmt.Errorf("create privacy consent for student %d: %w", studentID, err)
		}
		count++
	}

	fmt.Printf("  %d privacy consents created\n", count)
	return nil
}

// seedStatisticsDemoStep records a small, deterministic attendance and room
// scenario so a fresh demo tenant can show the Statistik page immediately.
type seedStatisticsDemoStep struct{}

func (seedStatisticsDemoStep) Name() string { return "Seeding statistics demo data" }

func (seedStatisticsDemoStep) Run(_ context.Context, rt *Runtime) (err error) {
	if rt.FixedSeeder == nil || len(rt.FixedSeeder.staffCredentials) == 0 {
		return fmt.Errorf("fixed seeder data not available")
	}
	deviceKey := rt.FixedSeeder.deviceKeys[DemoDevices[0].DeviceID]
	activityID := rt.FixedSeeder.activityIDs[DemoActivities[0].Name]
	roomID := rt.FixedSeeder.activityRoomIDs[activityID]
	// The final account is a guest and cannot use staff time tracking. The
	// penultimate account is regular staff, so the IoT session can create a
	// genuine NFC work-session block and the cleanup can close it through the
	// normal staff API. Small test fixtures contain only regular staff.
	supervisorIndex := max(0, len(rt.FixedSeeder.staffCredentials)-2)
	supervisor := rt.FixedSeeder.staffCredentials[supervisorIndex]
	staffID := rt.FixedSeeder.staffIDs[supervisor.Name]
	if deviceKey == "" || activityID == 0 || roomID == 0 || staffID == 0 {
		return fmt.Errorf("statistics demo prerequisites not available")
	}
	if _, err := rt.Client.DevicePost("/api/iot/session/start", map[string]any{
		"activity_id": activityID, "room_id": roomID, "supervisor_ids": []int64{staffID},
	}, deviceKey, rt.StaffPIN); err != nil {
		return fmt.Errorf("start statistics demo session: %w", err)
	}
	defer func() {
		cleanupErr := cleanUpStatisticsDemoSession(rt, deviceKey)
		if cleanupErr == nil {
			return
		}
		if err == nil {
			err = cleanupErr
			return
		}
		err = errors.Join(err, cleanupErr)
	}()
	seeded := 0
	for i := 0; i < 3 && i < len(DemoStudents); i++ {
		studentID, ok := rt.FixedSeeder.studentIDByIndex[i]
		if !ok {
			return fmt.Errorf("student ID not available for statistics demo index %d", i)
		}
		// RFID identifiers must be hexadecimal.
		rfid := fmt.Sprintf("57A7%08X", studentID)
		if _, err := rt.Client.DevicePost(fmt.Sprintf("/api/students/%d/rfid", studentID), map[string]string{"rfid_tag": rfid}, deviceKey, rt.StaffPIN); err != nil {
			return fmt.Errorf("assign statistics demo RFID: %w", err)
		}
		if _, err := rt.Client.DevicePost("/api/iot/attendance/toggle", map[string]string{"rfid": rfid, "action": "confirm"}, deviceKey, rt.StaffPIN); err != nil {
			return fmt.Errorf("record statistics demo attendance: %w", err)
		}
		if _, err := rt.Client.DevicePost("/api/iot/checkin", map[string]any{"student_rfid": rfid, "action": "checkin", "room_id": roomID}, deviceKey, rt.StaffPIN); err != nil {
			return fmt.Errorf("record statistics demo room visit: %w", err)
		}
		seeded++
	}
	fmt.Printf("  %d attendance records and room visits seeded for Statistik\n", seeded)
	return nil
}

func cleanUpStatisticsDemoSession(rt *Runtime, deviceKey string) error {
	var errs []error
	if _, err := rt.Client.DevicePost("/api/iot/session/end", nil, deviceKey, rt.StaffPIN); err != nil {
		errs = append(errs, fmt.Errorf("end statistics demo session: %w", err))
	}
	if err := checkOutStatisticsSupervisor(rt); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// checkOutStatisticsSupervisor bucht die Aufsicht wieder aus. Der
// Sitzungsstart stempelt sie per NFC ein (ensureNFCAutoCheckIn), das
// Sitzungsende bucht sie nicht aus; sonst bliebe eine offene Arbeitszeit im
// Mandanten stehen. Hat die Zeiterfassungs-Historie den heutigen Block schon
// geschrieben, unterbleibt der NFC-Stempel und es gibt nichts auszubuchen.
func checkOutStatisticsSupervisor(rt *Runtime) error {
	cred := rt.FixedSeeder.staffCredentials[max(0, len(rt.FixedSeeder.staffCredentials)-2)]
	previous := rt.Client.auth
	defer rt.Client.BindAuth(previous)

	if err := rt.Client.Login(cred.Email, cred.Password); err != nil {
		return fmt.Errorf("login statistics demo supervisor %s: %w", cred.Email, err)
	}
	open, err := hasOpenWorkSession(rt)
	if err != nil {
		return err
	}
	if !open {
		return nil
	}
	if _, err := rt.Client.Post("/api/time-tracking/check-out", nil); err != nil {
		return fmt.Errorf("check out statistics demo supervisor: %w", err)
	}
	return nil
}

// hasOpenWorkSession reports whether the logged-in staff account has a
// running work session. GET /current answers with a null payload when there
// is none.
func hasOpenWorkSession(rt *Runtime) (bool, error) {
	resp, err := rt.Client.Get("/api/time-tracking/current")
	if err != nil {
		return false, fmt.Errorf("read current work session: %w", err)
	}
	var payload struct {
		Data *struct {
			ID json.Number `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &payload); err != nil {
		return false, fmt.Errorf("parse current work session: %w", err)
	}
	return payload.Data != nil && payload.Data.ID != "", nil
}

// seedCareExitsStep records two planned ends of care (#2487) so the child
// management shows the "Betreuung endet am …" state and the reason table is
// not empty on a fresh machine.
//
// Both exits are dated today or later on purpose: the children stay fully
// usable for every demo and for `simulate full-day`, and the API refuses a
// retroactive exit anyway. The archive view ("Beendete Betreuungen") fills up
// by itself from the day after the first of them.
type seedCareExitsStep struct{}

func (seedCareExitsStep) Name() string { return "Seeding planned care exits" }

func (seedCareExitsStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}

	today := todaySeedDate()
	// Two children from the tail of the demo cohort, so the exits never
	// collide with the children the other steps mark sick or check in.
	plans := []struct {
		StudentIndex int
		LastCareDay  seedDate
		Reason       string
		Note         string
	}{
		{len(DemoStudents) - 1, today, "moved_away", ""},
		{len(DemoStudents) - 2, today.AddDays(7), "other", "Wechsel in die Nachmittagsbetreuung des Vereins"},
	}

	created := 0
	for _, plan := range plans {
		studentID, ok := rt.FixedSeeder.studentIDByIndex[plan.StudentIndex]
		if !ok {
			return fmt.Errorf("student ID not available for care-exit index %d", plan.StudentIndex)
		}
		body := map[string]any{
			"student_ids":   []string{strconv.FormatInt(studentID, 10)},
			"last_care_day": plan.LastCareDay.String(),
			"reason":        plan.Reason,
			"reason_note":   plan.Note,
		}

		// Preview first, exactly like the UI: the confirmation only accepts the
		// token the preview handed out.
		raw, err := rt.Client.Post("/api/students/care-end/preview", body)
		if err != nil {
			return fmt.Errorf("preview care exit for student %d: %w", studentID, err)
		}
		var preview struct {
			Data struct {
				Token   string `json:"token"`
				Blocked bool   `json:"blocked"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &preview); err != nil {
			return fmt.Errorf("decode POST /api/students/care-end/preview response for student %d: %w", studentID, err)
		}
		if preview.Data.Blocked {
			return fmt.Errorf("care-exit preview blocked for student %d", studentID)
		}
		if preview.Data.Token == "" {
			return fmt.Errorf("care-exit preview returned no token for student %d", studentID)
		}

		body["token"] = preview.Data.Token
		if _, err := rt.Client.Post("/api/students/care-end", body); err != nil {
			return fmt.Errorf("create care exit for student %d: %w", studentID, err)
		}
		created++
	}

	fmt.Printf("  %d planned care exits created\n", created)
	return nil
}
