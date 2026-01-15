package fixed

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/moto-nrw/project-phoenix/logging"
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

		_, err := s.tx.NewInsert().Model(group).
			ModelTableExpr("education.groups").
			On("CONFLICT (name) DO UPDATE").
			Set("room_id = EXCLUDED.room_id").
			Set(SQLExcludedUpdatedAt).
			Returning(SQLBaseColumns).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to upsert class group %s: %w", data.name, err)
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

		_, err := s.tx.NewInsert().Model(group).
			ModelTableExpr("education.groups").
			On("CONFLICT (name) DO UPDATE").
			Set("room_id = EXCLUDED.room_id").
			Set(SQLExcludedUpdatedAt).
			Returning(SQLBaseColumns).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to upsert supervision group %s: %w", data.name, err)
		}

		s.result.EducationGroups = append(s.result.EducationGroups, group)
		s.result.SupervisionGroups = append(s.result.SupervisionGroups, group)
		s.result.GroupByID[group.ID] = group
	}

	if s.verbose && logging.Logger != nil {
		logging.Logger.WithFields(map[string]any{
			"total":       len(s.result.EducationGroups),
			"classes":     len(s.result.ClassGroups),
			"supervision": len(s.result.SupervisionGroups),
		}).Info("Created education groups")
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

	// Ensure teacher 1 is assigned to group 3 (2A) for Bruno API tests
	// Group 3 is always "2A" (third in classData list)
	if len(s.result.Teachers) > 0 && len(s.result.ClassGroups) >= 3 {
		teacher1 := s.result.Teachers[0]  // First teacher (ID will be 1)
		group3 := s.result.ClassGroups[2] // Third class group "2A" (ID will be 3)

		groupTeacher := &education.GroupTeacher{
			GroupID:   group3.ID,
			TeacherID: teacher1.ID,
		}
		groupTeacher.CreatedAt = time.Now()
		groupTeacher.UpdatedAt = time.Now()

		_, err := s.tx.NewInsert().
			Model(groupTeacher).
			ModelTableExpr("education.group_teacher").
			On("CONFLICT (group_id, teacher_id) DO NOTHING").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to ensure teacher 1 assigned to group 3: %w", err)
		}

		if s.verbose && logging.Logger != nil {
			logging.Logger.WithFields(map[string]any{
				"teacher_id": teacher1.ID,
				"group_id":   group3.ID,
				"group_name": group3.Name,
			}).Debug("Ensured teacher assigned to group for API tests")
		}
	}

	if s.verbose && logging.Logger != nil {
		logging.Logger.Info("Assigned teachers to all education groups")
	}

	return nil
}
