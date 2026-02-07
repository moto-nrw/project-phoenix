package fixed

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moto-nrw/project-phoenix/models/platform"
)

// seedAnnouncements creates sample announcements for development
func (s *Seeder) seedAnnouncements(ctx context.Context) error {
	if len(s.result.Operators) == 0 {
		return fmt.Errorf("no operators found - cannot create announcements")
	}

	operatorID := s.result.Operators[0].ID
	now := time.Now()
	version := "2.1.0"

	announcements := []platform.Announcement{
		{
			Title:       "Willkommen bei moto!",
			Content:     "Herzlich willkommen zur neuen moto Plattform. Hier finden Sie alle wichtigen Informationen und Updates rund um die OGS-Verwaltung. Bei Fragen wenden Sie sich bitte an das Support-Team.",
			Type:        platform.TypeAnnouncement,
			Severity:    platform.SeverityInfo,
			Active:      true,
			PublishedAt: &now,
			CreatedBy:   operatorID,
			TargetRoles: []string{}, // All roles
		},
		{
			Title:       "Neue Version 2.1.0 verfügbar",
			Content:     "Mit dieser Version wurden folgende Verbesserungen eingeführt:\n\n• Verbesserte Anwesenheitsübersicht\n• Neue Filterfunktionen für Gruppen\n• Optimierte Performance beim Laden von Schülerdaten\n• Bugfixes und Stabilitätsverbesserungen",
			Type:        platform.TypeRelease,
			Severity:    platform.SeverityInfo,
			Version:     &version,
			Active:      true,
			PublishedAt: &now,
			CreatedBy:   operatorID,
			TargetRoles: []string{}, // All roles
		},
		{
			Title:       "Geplante Wartungsarbeiten am Wochenende",
			Content:     "Am kommenden Samstag (08:00 - 12:00 Uhr) finden Wartungsarbeiten am Server statt. In dieser Zeit kann es zu kurzen Unterbrechungen kommen. Wir bitten um Verständnis.",
			Type:        platform.TypeMaintenance,
			Severity:    platform.SeverityWarning,
			Active:      true,
			PublishedAt: &now,
			CreatedBy:   operatorID,
			TargetRoles: []string{platform.RoleAdmin, platform.RoleUser},
		},
		{
			Title:       "Wichtig: Datenschutz-Update erforderlich",
			Content:     "Aufgrund neuer DSGVO-Anforderungen müssen alle Nutzer ihre Datenschutzeinstellungen bis zum Monatsende überprüfen und bestätigen. Bitte loggen Sie sich ein und aktualisieren Sie Ihre Einstellungen im Profil-Bereich.",
			Type:        platform.TypeAnnouncement,
			Severity:    platform.SeverityCritical,
			Active:      true,
			PublishedAt: &now,
			CreatedBy:   operatorID,
			TargetRoles: []string{}, // All roles
		},
		{
			Title:       "Entwurf: Neue Funktionen in Planung",
			Content:     "Wir arbeiten an spannenden neuen Funktionen für das nächste Update. Mehr Details folgen in Kürze.",
			Type:        platform.TypeAnnouncement,
			Severity:    platform.SeverityInfo,
			Active:      true,
			PublishedAt: nil, // Draft - not published
			CreatedBy:   operatorID,
			TargetRoles: []string{},
		},
	}

	for i := range announcements {
		ann := &announcements[i]
		ann.CreatedAt = now.Add(-time.Duration(len(announcements)-i) * 24 * time.Hour) // Stagger creation dates
		ann.UpdatedAt = ann.CreatedAt

		_, err := s.tx.NewInsert().Model(ann).
			ModelTableExpr("platform.announcements").
			Returning(SQLBaseColumns).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create announcement %q: %w", ann.Title, err)
		}

		s.result.Announcements = append(s.result.Announcements, ann)
	}

	if s.verbose {
		log.Printf("Created %d announcement(s)", len(s.result.Announcements))
	}

	return nil
}
