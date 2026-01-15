package fixed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// Activity block name constants to avoid duplicate string literals
const (
	ActivityBlock1 = "ActivityBlock1"
	ActivityBlock2 = "ActivityBlock2"
)

// Activity data for seeding
type activityGroupData struct {
	name            string
	description     string
	category        string
	maxParticipants int
	minAge          int
	maxAge          int
	requiresConsent bool
	roomName        string
}

// seedActivities creates activity categories and groups
func (s *Seeder) seedActivities(ctx context.Context) error {
	// Create activity categories
	categoryData := []struct {
		name        string
		description string
		color       string
	}{
		{"Sport", "Sportliche Aktivitäten für Kinder", "#7ED321"},
		{"Kunst & Basteln", "Kreative Aktivitäten und Handwerken", "#F5A623"},
		{"Musik", "Musikalische Aktivitäten und Gesang", "#BD10E0"},
		{"Spiele", "Brett-, Karten- und Gruppenspiele", "#50E3C2"},
		{"Lesen", "Leseförderung und Literatur", "#B8E986"},
		{"Hausaufgabenhilfe", "Unterstützung bei den Hausaufgaben", "#4A90E2"},
		{"Natur & Forschen", "Naturerkundung und einfache Experimente", "#7ED321"},
		{"Computer", "Grundlagen im Umgang mit dem Computer", "#9013FE"},
		{"Schulhof", "Schulhof und Außenbereich", "#7ED321"},
	}

	for _, data := range categoryData {
		category := &activities.Category{
			Name:        data.name,
			Description: data.description,
			Color:       data.color,
		}
		category.CreatedAt = time.Now()
		category.UpdatedAt = time.Now()

		_, err := s.tx.NewInsert().Model(category).
			ModelTableExpr("activities.categories").
			On("CONFLICT (name) DO UPDATE").
			Set(SQLExcludedUpdatedAt).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create activity category %s: %w", data.name, err)
		}

		// Reload to get ID
		err = s.tx.NewRaw("SELECT * FROM activities.categories WHERE name = ?", data.name).Scan(ctx, category)
		if err != nil {
			return fmt.Errorf("failed to reload category %s: %w", data.name, err)
		}

		s.result.ActivityCategories = append(s.result.ActivityCategories, category)
	}

	// Create activity groups
	activityGroups := []activityGroupData{
		// Sport
		{"Fußball-AG", "Fußballtraining für Anfänger und Fortgeschrittene", "Sport", 20, 6, 12, false, "Sporthalle"},
		{"Basketball für Anfänger", "Grundlagen des Basketballspiels", "Sport", 15, 8, 12, false, "Sporthalle"},
		{"Tanz und Bewegung", "Moderne Tänze und Bewegungsspiele", "Sport", 20, 6, 10, false, "Kleine Sporthalle"},
		{"Schwimmen", "Schwimmkurs für Fortgeschrittene", "Sport", 12, 8, 12, true, "Sporthalle"}, // Requires consent

		// Kunst & Basteln
		{"Basteln und Malen", "Kreatives Gestalten mit verschiedenen Materialien", "Kunst & Basteln", 15, 6, 10, false, "Kunstraum"},
		{"Töpfern", "Arbeiten mit Ton und Keramik", "Kunst & Basteln", 10, 8, 12, false, "Töpferraum"},
		{"Origami-Werkstatt", "Papierfalten nach japanischer Tradition", "Kunst & Basteln", 12, 8, 12, false, "Kunstraum"},

		// Musik
		{"Kinderchor", "Gemeinsames Singen und Stimmbildung", "Musik", 25, 6, 12, false, "Musikraum"},
		{"Rhythmus und Percussion", "Trommeln und Rhythmusinstrumente", "Musik", 15, 6, 10, false, "Musikraum"},
		{"Blockflöten-AG", "Blockflötenunterricht für Anfänger", "Musik", 10, 7, 10, false, "Musikraum"},

		// Spiele
		{"Schach für Anfänger", "Grundlagen des Schachspiels", "Spiele", 16, 8, 12, false, "Bibliothek"},
		{"Brett- und Kartenspiele", "Verschiedene Gesellschaftsspiele", "Spiele", 20, 6, 12, false, "OGS-Raum 2"},

		// Lesen
		{"Leseclub", "Gemeinsames Lesen und Buchbesprechungen", "Lesen", 15, 8, 12, false, "Bibliothek"},
		{"Vorlesestunde", "Geschichten für die Jüngeren", "Lesen", 20, 6, 8, false, "Bibliothek"},

		// Hausaufgabenhilfe
		{"Hausaufgaben Klasse 1-2", "Betreute Hausaufgabenzeit", "Hausaufgabenhilfe", 20, 6, 8, false, "OGS-Raum 1"},
		{"Hausaufgaben Klasse 3-4", "Betreute Hausaufgabenzeit", "Hausaufgabenhilfe", 20, 8, 10, false, "Klassenzimmer 3A"},
		{"Hausaufgaben Klasse 5", "Betreute Hausaufgabenzeit", "Hausaufgabenhilfe", 15, 10, 12, false, "Klassenzimmer 5A"},

		// Natur & Forschen
		{"Junge Forscher", "Experimente und Naturbeobachtungen", "Natur & Forschen", 12, 8, 12, false, "Forscherraum"},

		// Computer
		{"Computer-Grundlagen", "Erste Schritte am Computer", "Computer", 15, 8, 12, false, "Computerraum"},

		// Schulhof
		{constants.SchulhofActivityName, "Freies Spiel im Schulhof", "Schulhof", 100, 6, 12, false, "Schulhof"},
	}

	// Map category names to IDs
	categoryMap := make(map[string]int64)
	for _, cat := range s.result.ActivityCategories {
		categoryMap[cat.Name] = cat.ID
	}

	// Create activity groups
	for _, data := range activityGroups {
		// Find room
		var roomID *int64
		for _, room := range s.result.Rooms {
			if room.Name == data.roomName {
				roomID = &room.ID
				break
			}
		}

		existing := new(activities.Group)
		err := s.tx.NewSelect().Model(existing).
			ModelTableExpr(`activities.groups AS "group"`).
			Where("name = ?", data.name).
			Limit(1).
			Scan(ctx)

		switch {
		case err == nil:
			existing.CategoryID = categoryMap[data.category]
			existing.MaxParticipants = data.maxParticipants
			existing.PlannedRoomID = roomID
			existing.IsOpen = true
			existing.UpdatedAt = time.Now()

			if _, err := s.tx.NewUpdate().Model(existing).
				ModelTableExpr(`activities.groups AS "group"`).
				Column("category_id", "max_participants", "planned_room_id", "is_open", "updated_at").
				WherePK().
				Exec(ctx); err != nil {
				return fmt.Errorf("failed to update activity group %s: %w", data.name, err)
			}

			s.result.ActivityGroups = append(s.result.ActivityGroups, existing)
			s.result.ActivityByID[existing.ID] = existing

		case errors.Is(err, sql.ErrNoRows):
			group := &activities.Group{
				Name:            data.name,
				CategoryID:      categoryMap[data.category],
				MaxParticipants: data.maxParticipants,
				PlannedRoomID:   roomID,
				IsOpen:          true,
			}
			group.CreatedAt = time.Now()
			group.UpdatedAt = group.CreatedAt

			if _, err := s.tx.NewInsert().Model(group).
				ModelTableExpr(`activities.groups AS "group"`).
				Returning(SQLBaseColumns).
				Exec(ctx); err != nil {
				return fmt.Errorf("failed to create activity group %s: %w", data.name, err)
			}

			s.result.ActivityGroups = append(s.result.ActivityGroups, group)
			s.result.ActivityByID[group.ID] = group

		default:
			return fmt.Errorf("failed to load activity group %s: %w", data.name, err)
		}
	}

	// Assign supervisors to activities
	if err := s.assignActivitySupervisors(ctx); err != nil {
		return fmt.Errorf("failed to assign activity supervisors: %w", err)
	}

	if s.verbose && logging.Logger != nil {
		logging.Logger.WithFields(map[string]any{
			"categories": len(s.result.ActivityCategories),
			"groups":     len(s.result.ActivityGroups),
		}).Info("Created activity categories and groups")
	}

	return nil
}

// assignActivitySupervisors assigns staff to supervise activities
func (s *Seeder) assignActivitySupervisors(ctx context.Context) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Each activity needs 1-2 supervisors
	for _, activity := range s.result.ActivityGroups {
		// Delete existing supervisors for this activity to ensure idempotent seeding
		_, err := s.tx.NewDelete().
			Table("activities.supervisors").
			Where("group_id = ?", activity.ID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to clear supervisors for activity %d: %w", activity.ID, err)
		}

		numSupervisors := rng.Intn(2) + 1 // 1 or 2

		// Create shuffled staff list
		staffIndices := make([]int, len(s.result.Staff))
		for i := range staffIndices {
			staffIndices[i] = i
		}
		// Shuffle
		for i := len(staffIndices) - 1; i > 0; i-- {
			j := rng.Intn(i + 1)
			staffIndices[i], staffIndices[j] = staffIndices[j], staffIndices[i]
		}

		// Assign supervisors
		for i := 0; i < numSupervisors && i < len(staffIndices); i++ {
			staff := s.result.Staff[staffIndices[i]]

			assignment := &activities.SupervisorPlanned{
				GroupID:   activity.ID,
				StaffID:   staff.ID,
				IsPrimary: i == 0, // First supervisor is primary
			}
			assignment.CreatedAt = time.Now()
			assignment.UpdatedAt = time.Now()

			_, err := s.tx.NewInsert().Model(assignment).
				ModelTableExpr("activities.supervisors").
				On("CONFLICT (group_id, staff_id) DO UPDATE").
				Set("is_primary = EXCLUDED.is_primary").
				Set(SQLExcludedUpdatedAt).
				Returning("created_at, updated_at").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to assign supervisor to activity %d: %w",
					activity.ID, err)
			}
		}
	}

	return nil
}

// seedTimeframes creates the daily schedule timeframes
func (s *Seeder) seedTimeframes(ctx context.Context) error {
	// OGS timeframes for elementary school afternoon supervision
	timeframeData := []struct {
		description string
		startHour   int
		startMinute int
		endHour     int
		endMinute   int
	}{
		{"Mittagessen", 12, 0, 13, 0},
		{"Freispiel/Ruhephase", 13, 0, 14, 0},
		{ActivityBlock1, 14, 0, 15, 0},
		{"Pause", 15, 0, 15, 15},
		{ActivityBlock2, 15, 15, 16, 15},
		{"Freispiel/Abholzeit", 16, 15, 17, 0},
	}

	today := time.Now()
	for _, data := range timeframeData {
		// Create start and end times
		startTime := time.Date(today.Year(), today.Month(), today.Day(),
			data.startHour, data.startMinute, 0, 0, time.Local)
		endTime := time.Date(today.Year(), today.Month(), today.Day(),
			data.endHour, data.endMinute, 0, 0, time.Local)

		existing := new(schedule.Timeframe)
		err := s.tx.NewSelect().Model(existing).
			ModelTableExpr(`schedule.timeframes AS "timeframe"`).
			Where("description = ?", data.description).
			Limit(1).
			Scan(ctx)

		switch {
		case err == nil:
			existing.StartTime = startTime
			existing.EndTime = &endTime
			existing.UpdatedAt = time.Now()

			if _, err := s.tx.NewUpdate().Model(existing).
				ModelTableExpr(`schedule.timeframes AS "timeframe"`).
				Column("start_time", "end_time", "updated_at").
				WherePK().
				Exec(ctx); err != nil {
				return fmt.Errorf("failed to update timeframe %s: %w", data.description, err)
			}
			s.result.Timeframes = append(s.result.Timeframes, existing)

		case errors.Is(err, sql.ErrNoRows):
			timeframe := &schedule.Timeframe{
				Description: data.description,
				StartTime:   startTime,
				EndTime:     &endTime,
			}
			timeframe.CreatedAt = time.Now()
			timeframe.UpdatedAt = timeframe.CreatedAt

			if _, err := s.tx.NewInsert().Model(timeframe).
				ModelTableExpr(`schedule.timeframes AS "timeframe"`).
				Returning(SQLBaseColumns).
				Exec(ctx); err != nil {
				return fmt.Errorf("failed to create timeframe %s: %w", data.description, err)
			}
			s.result.Timeframes = append(s.result.Timeframes, timeframe)

		default:
			return fmt.Errorf("failed to load timeframe %s: %w", data.description, err)
		}
	}

	if s.verbose && logging.Logger != nil {
		logging.Logger.WithField("count", len(s.result.Timeframes)).Info("Created schedule timeframes")
	}

	return nil
}

// seedActivitySchedules links activities to timeframes and weekdays
func (s *Seeder) seedActivitySchedules(ctx context.Context) error {
	// Map activities to their schedule slots
	scheduleMap := map[string]struct {
		timeframeDesc string
		weekdays      []string
	}{
		// Hausaufgaben during first slot
		"Hausaufgaben Klasse 1-2": {ActivityBlock1, []string{"monday", "tuesday", "wednesday", "thursday", "friday"}},
		"Hausaufgaben Klasse 3-4": {ActivityBlock1, []string{"monday", "tuesday", "wednesday", "thursday", "friday"}},
		"Hausaufgaben Klasse 5":   {ActivityBlock1, []string{"monday", "tuesday", "wednesday", "thursday", "friday"}},

		// Sport activities
		"Fußball-AG":              {ActivityBlock2, []string{"monday", "wednesday", "friday"}},
		"Basketball für Anfänger": {ActivityBlock2, []string{"tuesday", "thursday"}},
		"Tanz und Bewegung":       {ActivityBlock1, []string{"tuesday", "thursday"}},
		"Schwimmen":               {ActivityBlock2, []string{"wednesday"}},

		// Creative activities
		"Basteln und Malen": {ActivityBlock2, []string{"monday", "wednesday"}},
		"Töpfern":           {ActivityBlock2, []string{"tuesday", "thursday"}},
		"Origami-Werkstatt": {ActivityBlock1, []string{"friday"}},

		// Music
		"Kinderchor":              {ActivityBlock2, []string{"tuesday", "thursday"}},
		"Rhythmus und Percussion": {ActivityBlock1, []string{"monday", "wednesday"}},
		"Blockflöten-AG":          {ActivityBlock2, []string{"friday"}},

		// Other activities
		"Schach für Anfänger":     {ActivityBlock2, []string{"monday", "friday"}},
		"Brett- und Kartenspiele": {"Freispiel/Ruhephase", []string{"monday", "tuesday", "wednesday", "thursday", "friday"}},
		"Leseclub":                {ActivityBlock1, []string{"wednesday"}},
		"Vorlesestunde":           {"Freispiel/Ruhephase", []string{"tuesday", "thursday"}},
		"Junge Forscher":          {ActivityBlock2, []string{"wednesday", "friday"}},
		"Computer-Grundlagen":     {ActivityBlock2, []string{"tuesday", "thursday"}},
	}

	// Map timeframe descriptions to IDs
	timeframeMap := make(map[string]int64)
	for _, tf := range s.result.Timeframes {
		timeframeMap[tf.Description] = tf.ID
	}

	// Create schedules
	for _, activity := range s.result.ActivityGroups {
		if schedule, ok := scheduleMap[activity.Name]; ok {
			timeframeID := timeframeMap[schedule.timeframeDesc]

			for _, weekday := range schedule.weekdays {
				weekdayInt := weekdayToInt(weekday)
				sched := &activities.Schedule{
					ActivityGroupID: activity.ID,
					TimeframeID:     &timeframeID,
					Weekday:         weekdayInt,
				}
				sched.CreatedAt = time.Now()
				sched.UpdatedAt = time.Now()

				_, err := s.tx.NewInsert().Model(sched).
					ModelTableExpr("activities.schedules").
					On("CONFLICT (weekday, timeframe_id, activity_group_id) WHERE (timeframe_id IS NOT NULL) DO UPDATE").
					Set(SQLExcludedUpdatedAt).
					Returning(SQLBaseColumns).
					Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to upsert schedule for activity %s: %w",
						activity.Name, err)
				}

				s.result.Schedules = append(s.result.Schedules, sched)
			}
		}
	}

	if s.verbose && logging.Logger != nil {
		logging.Logger.WithField("count", len(s.result.Schedules)).Info("Created activity schedules")
	}

	return nil
}

// seedStudentEnrollments enrolls students in activities
func (s *Seeder) seedStudentEnrollments(ctx context.Context) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Track enrollments per activity
	enrollmentCounts := make(map[int64]int)
	for _, activity := range s.result.ActivityGroups {
		enrollmentCounts[activity.ID] = 0
	}

	// Each student gets 2-3 activities
	for _, student := range s.result.Students {
		numActivities := rng.Intn(2) + 2 // 2 or 3 activities

		// Get student's age based on class (not used anymore)
		// studentAge := getStudentAge(student.SchoolClass)

		// Find eligible activities
		eligibleActivities := []*activities.Group{}
		for _, activity := range s.result.ActivityGroups {
			// For now, all activities are eligible (no age restrictions in model)
			// Check capacity (70-85% fill rate)
			fillRate := float64(enrollmentCounts[activity.ID]) / float64(activity.MaxParticipants)
			if fillRate < 0.85 {
				eligibleActivities = append(eligibleActivities, activity)
			}
		}

		// Shuffle eligible activities
		for i := len(eligibleActivities) - 1; i > 0; i-- {
			j := rng.Intn(i + 1)
			eligibleActivities[i], eligibleActivities[j] = eligibleActivities[j], eligibleActivities[i]
		}

		// Enroll in activities
		enrolled := 0
		for _, activity := range eligibleActivities {
			if enrolled >= numActivities {
				break
			}

			// No consent check needed (not in model)

			exists, err := s.tx.NewSelect().
				Table("activities.student_enrollments").
				Where("student_id = ? AND activity_group_id = ?", student.ID, activity.ID).
				Exists(ctx)
			if err != nil {
				return fmt.Errorf("failed to check existing enrollment: %w", err)
			}
			if exists {
				enrolled++
				enrollmentCounts[activity.ID]++
				continue
			}

			enrollment := &activities.StudentEnrollment{
				StudentID:       student.ID,
				ActivityGroupID: activity.ID,
				EnrollmentDate:  time.Now().AddDate(0, 0, -rng.Intn(30)),
			}
			enrollment.CreatedAt = time.Now()
			enrollment.UpdatedAt = time.Now()

			_, err = s.tx.NewInsert().Model(enrollment).
				ModelTableExpr("activities.student_enrollments").
				Returning(SQLBaseColumns).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to create student enrollment: %w", err)
			}

			s.result.Enrollments = append(s.result.Enrollments, enrollment)
			enrollmentCounts[activity.ID]++
			enrolled++
		}
	}

	if s.verbose && logging.Logger != nil {
		logging.Logger.WithField("count", len(s.result.Enrollments)).Info("Created student enrollments")

		// Log fill rates
		for _, activity := range s.result.ActivityGroups {
			count := enrollmentCounts[activity.ID]
			fillRate := float64(count) / float64(activity.MaxParticipants) * 100
			logging.Logger.WithFields(map[string]any{
				"activity":     activity.Name,
				"enrolled":     count,
				"max":          activity.MaxParticipants,
				"fill_rate":    fillRate,
			}).Debug("Activity fill rate")
		}
	}

	return nil
}

// Helper function to estimate student age based on class
// Unused - kept for potential future use
// func getStudentAge(class string) int {
// 	if len(class) < 1 {
// 		return 8 // Default
// 	}
//
// 	grade := class[0] - '0'
// 	if grade >= 1 && grade <= 5 {
// 		return int(grade) + 5 // Grade 1 = 6 years old, Grade 5 = 10 years old
// 	}
//
// 	return 8 // Default
// }

// Helper function to convert weekday string to int
func weekdayToInt(weekday string) int {
	switch weekday {
	case "monday":
		return activities.WeekdayMonday
	case "tuesday":
		return activities.WeekdayTuesday
	case "wednesday":
		return activities.WeekdayWednesday
	case "thursday":
		return activities.WeekdayThursday
	case "friday":
		return activities.WeekdayFriday
	case "saturday":
		return activities.WeekdaySaturday
	case "sunday":
		return activities.WeekdaySunday
	default:
		return activities.WeekdayMonday // Default
	}
}
