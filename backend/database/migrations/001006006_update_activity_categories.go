package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	UpdateActivityCategoriesVersion     = "1.6.6"
	UpdateActivityCategoriesDescription = "Update activity category names and add Mensa category"
)

func init() {
	MigrationRegistry[UpdateActivityCategoriesVersion] = &Migration{
		Version:     UpdateActivityCategoriesVersion,
		Description: UpdateActivityCategoriesDescription,
		DependsOn:   []string{SeedActivityCategoriesVersion}, // 1.6.4
	}

	Migrations.MustRegister(func(_ context.Context, db *bun.DB) error {
		// Update existing categories to new names and descriptions
		categoryUpdates := []struct {
			oldName        string
			newName        string
			newDescription string
		}{
			{"Kunst & Basteln", "Kreativ", "Kreative Aktivitäten und Handwerken"},
			{"Lesen", "Lernen", "Lernförderung und Bildungsaktivitäten"},
			{"Hausaufgabenhilfe", "Hausaufgaben", "Unterstützung bei den Hausaufgaben"},
			{"Natur & Forschen", "Draußen", "Aktivitäten im Freien und Naturerkundung"},
		}

		for _, update := range categoryUpdates {
			_, err := db.Exec("UPDATE activities.categories SET name = ?, description = ? WHERE name = ?",
				update.newName, update.newDescription, update.oldName)
			if err != nil {
				return err
			}
		}

		// Remove the Computer category
		_, err := db.Exec("DELETE FROM activities.categories WHERE name = ?", "Computer")
		if err != nil {
			return err
		}

		// Add new Mensa category
		_, err = db.Exec(`
			INSERT INTO activities.categories (name, description, color, created_at, updated_at)
			VALUES (?, ?, ?, NOW(), NOW())`,
			"Mensa", "Aktivitäten rund um das Mittagessen", "#FF9500")
		if err != nil {
			return err
		}

		return nil
	}, func(_ context.Context, db *bun.DB) error {
		_, err := db.Exec(rollback001006006)
		return err
	})
}

var rollback001006006 = `
-- Revert category updates back to original names
UPDATE activities.categories SET name = 'Sport', description = 'Sportliche Aktivitäten für Kinder' WHERE name = 'Sport';
UPDATE activities.categories SET name = 'Kunst & Basteln', description = 'Kreative Aktivitäten und Handwerken' WHERE name = 'Kreativ';
UPDATE activities.categories SET name = 'Musik', description = 'Musikalische Aktivitäten und Gesang' WHERE name = 'Musik';
UPDATE activities.categories SET name = 'Spiele', description = 'Brett-, Karten- und Gruppenspiele' WHERE name = 'Spiele';
UPDATE activities.categories SET name = 'Lesen', description = 'Leseförderung und Literatur' WHERE name = 'Lernen';
UPDATE activities.categories SET name = 'Hausaufgabenhilfe', description = 'Unterstützung bei den Hausaufgaben' WHERE name = 'Hausaufgaben';
UPDATE activities.categories SET name = 'Natur & Forschen', description = 'Naturerkundung und einfache Experimente' WHERE name = 'Draußen';
UPDATE activities.categories SET name = 'Computer', description = 'Grundlagen im Umgang mit dem Computer' WHERE name = 'Computer';
UPDATE activities.categories SET name = 'Gruppenraum', description = 'Aktivitäten im Gruppenraum' WHERE name = 'Gruppenraum';

-- Remove new Mensa category
DELETE FROM activities.categories WHERE name = 'Mensa';
`
