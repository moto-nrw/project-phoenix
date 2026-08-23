package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// seedAnnouncementsStep creates demo announcements and one published parent
// letter so the delivery-status matrix has realistic seed data.
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

	for _, a := range announcements {
		if _, err := rt.Client.Post("/operator/announcements", a); err != nil {
			fmt.Printf("  WARNING: failed to create announcement %q: %v\n", a["title"], err)
			continue
		}
	}

	rt.Client.BindAuth(rt.TenantAuth)
	if err := seedParentLetter(rt); err != nil {
		return err
	}

	fmt.Printf("  %d announcements created\n", len(announcements))
	return nil
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
	if _, err := rt.Client.Post(fmt.Sprintf("/api/parent-announcements/%d/publish", id), nil); err != nil {
		return fmt.Errorf("publish parent letter: %w", err)
	}
	fmt.Println("  1 parent letter published")
	return nil
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
			continue
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
			if rt.Verbose {
				fmt.Printf("  WARNING: failed to create consent for student %d: %v\n", studentID, err)
			}
			continue
		}
		count++
	}

	fmt.Printf("  %d privacy consents created\n", count)
	return nil
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

	today := timezone.TodayDate()
	// Two children from the tail of the demo cohort, so the exits never
	// collide with the children the other steps mark sick or check in.
	plans := []struct {
		StudentIndex int
		LastCareDay  timezone.Date
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
			continue
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
			if rt.Verbose {
				fmt.Printf("  WARNING: care-exit preview failed for student %d: %v\n", studentID, err)
			}
			continue
		}
		var preview struct {
			Data struct {
				Token   string `json:"token"`
				Blocked bool   `json:"blocked"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &preview); err != nil || preview.Data.Blocked || preview.Data.Token == "" {
			if rt.Verbose {
				fmt.Printf("  WARNING: care-exit preview unusable for student %d\n", studentID)
			}
			continue
		}

		body["token"] = preview.Data.Token
		if _, err := rt.Client.Post("/api/students/care-end", body); err != nil {
			if rt.Verbose {
				fmt.Printf("  WARNING: care-exit failed for student %d: %v\n", studentID, err)
			}
			continue
		}
		created++
	}

	fmt.Printf("  %d planned care exits created\n", created)
	return nil
}
