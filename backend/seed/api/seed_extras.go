package api

import (
	"context"
	"fmt"
	"time"
)

// seedAnnouncementsStep creates demo announcements via the operator API.
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

	fmt.Printf("  %d announcements created\n", len(announcements))
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
