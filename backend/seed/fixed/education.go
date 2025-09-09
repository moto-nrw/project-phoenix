package fixed

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/moto-nrw/project-phoenix/models/education"
)

// seedEducationGroups creates education groups (classes and supervision groups)
func (s *Seeder) seedEducationGroups(ctx context.Context) error {
	// Class groups (10 classes, 2 per grade)
	classData := []struct {
		name        string
		description string
		roomName    string
	}{
		{"1A", "Klasse 1A - Erstklässler", "Klassenzimmer 1A"},
		{"1B", "Klasse 1B - Erstklässler", "Klassenzimmer 1B"},
		{"2A", "Klasse 2A - Zweitklässler", "Klassenzimmer 2A"},
		{"2B", "Klasse 2B - Zweitklässler", "Klassenzimmer 2B"},
		{"3A", "Klasse 3A - Drittklässler", "Klassenzimmer 3A"},
		{"3B", "Klasse 3B - Drittklässler", "Klassenzimmer 3B"},
		{"4A", "Klasse 4A - Viertklässler", "Klassenzimmer 4A"},
		{"4B", "Klasse 4B - Viertklässler", "Klassenzimmer 4B"},
		{"5A", "Klasse 5A - Fünftklässler", "Klassenzimmer 5A"},
		{"5B", "Klasse 5B - Fünftklässler", "Klassenzimmer 5B"},
	}

	// Create class groups
	for _, data := range classData {
		// Find the room
		var roomID *int64
		for _, room := range s.result.Rooms {
			if room.Name == data.roomName {
				roomID = &room.ID
				break
			}
		}

		group := &education.Group{
			Name:   data.name,
			RoomID: roomID,
		}
		group.CreatedAt = time.Now()
		group.UpdatedAt = time.Now()

		_, err := s.tx.NewInsert().Model(group).ModelTableExpr("education.groups").Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create class group %s: %w", data.name, err)
		}

		s.result.EducationGroups = append(s.result.EducationGroups, group)
		s.result.ClassGroups = append(s.result.ClassGroups, group)
		s.result.GroupByID[group.ID] = group
	}

	// OGS supervision groups
	supervisionData := []struct {
		name        string
		description string
		roomName    string
	}{
		{"OGS-Früh", "Frühbetreuung 7:00-8:00", "OGS-Raum 1"},
		{"OGS-Mittag-1", "Mittagsbetreuung Gruppe 1", "Mensa"},
		{"OGS-Mittag-2", "Mittagsbetreuung Gruppe 2", "Mensa"},
		{"OGS-Hausaufgaben-1", "Hausaufgabenbetreuung Gruppe 1", "OGS-Raum 1"},
		{"OGS-Hausaufgaben-2", "Hausaufgabenbetreuung Gruppe 2", "OGS-Raum 2"},
		{"OGS-Hausaufgaben-3", "Hausaufgabenbetreuung Gruppe 3", "Klassenzimmer 1A"},
		{"OGS-Freispiel-1", "Freispiel und Bewegung", "Sporthalle"},
		{"OGS-Freispiel-2", "Freispiel und Ruhe", "OGS-Raum 2"},
		{"OGS-Förderung", "Individuelle Förderung", "Bibliothek"},
		{"OGS-Kreativ", "Kreativwerkstatt", "Kunstraum"},
		{"OGS-Musik", "Musikalische Früherziehung", "Musikraum"},
		{"OGS-Computer", "Computer-AG", "Computerraum"},
		{"OGS-Werken", "Werken und Basteln", "Werkraum"},
		{"OGS-Natur", "Natur und Umwelt", "Forscherraum"},
		{"OGS-Spät", "Spätbetreuung 16:00-17:00", "OGS-Raum 1"},
	}

	// Create supervision groups
	for _, data := range supervisionData {
		// Find the room
		var roomID *int64
		for _, room := range s.result.Rooms {
			if room.Name == data.roomName {
				roomID = &room.ID
				break
			}
		}

		group := &education.Group{
			Name:   data.name,
			RoomID: roomID,
		}
		group.CreatedAt = time.Now()
		group.UpdatedAt = time.Now()

		_, err := s.tx.NewInsert().Model(group).ModelTableExpr("education.groups").Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create supervision group %s: %w", data.name, err)
		}

		s.result.EducationGroups = append(s.result.EducationGroups, group)
		s.result.SupervisionGroups = append(s.result.SupervisionGroups, group)
		s.result.GroupByID[group.ID] = group
	}

	if s.verbose {
		log.Printf("Created %d education groups (%d classes, %d supervision)",
			len(s.result.EducationGroups),
			len(s.result.ClassGroups),
			len(s.result.SupervisionGroups))
	}

	return nil
}

// assignTeachersToGroups creates group-teacher relationships
func (s *Seeder) assignTeachersToGroups(ctx context.Context) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Assign 1-2 teachers to each education group
	for _, group := range s.result.EducationGroups {
		// Number of teachers for this group
		numTeachers := rng.Intn(2) + 1 // 1 or 2 teachers

		// Create a shuffled list of teacher indices
		teacherIndices := make([]int, len(s.result.Teachers))
		for i := range teacherIndices {
			teacherIndices[i] = i
		}
		// Shuffle
		for i := len(teacherIndices) - 1; i > 0; i-- {
			j := rng.Intn(i + 1)
			teacherIndices[i], teacherIndices[j] = teacherIndices[j], teacherIndices[i]
		}

		// Assign teachers
		for i := 0; i < numTeachers && i < len(teacherIndices); i++ {
			teacher := s.result.Teachers[teacherIndices[i]]

			// Create group-teacher relationship
			groupTeacher := &education.GroupTeacher{
				GroupID:   group.ID,
				TeacherID: teacher.ID,
			}
			groupTeacher.CreatedAt = time.Now()
			groupTeacher.UpdatedAt = time.Now()

			_, err := s.tx.NewInsert().
				Model(groupTeacher).
				ModelTableExpr("education.group_teacher").
				On("CONFLICT (group_id, teacher_id) DO NOTHING").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to assign teacher %d to group %d: %w",
					teacher.ID, group.ID, err)
			}
		}
	}

	if s.verbose {
		log.Printf("Assigned teachers to all education groups")
	}

	return nil
}