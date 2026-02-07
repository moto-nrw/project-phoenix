package config

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// DefaultObjectRefResolver resolves object references to database entities
type DefaultObjectRefResolver struct {
	db *bun.DB
}

// NewDefaultObjectRefResolver creates a new object reference resolver
func NewDefaultObjectRefResolver(db *bun.DB) *DefaultObjectRefResolver {
	return &DefaultObjectRefResolver{db: db}
}

// ResolveOptions returns available options for an object reference type
func (r *DefaultObjectRefResolver) ResolveOptions(ctx context.Context, refType string, filter map[string]interface{}) ([]*config.ObjectRefOption, error) {
	switch refType {
	case "room":
		return r.resolveRooms(ctx, filter)
	case "group":
		return r.resolveGroups(ctx, filter)
	case "staff":
		return r.resolveStaff(ctx, filter)
	case "teacher":
		return r.resolveTeachers(ctx, filter)
	case "student":
		return r.resolveStudents(ctx, filter)
	case "activity":
		return r.resolveActivities(ctx, filter)
	case "device":
		return r.resolveDevices(ctx, filter)
	default:
		return nil, fmt.Errorf("unknown object reference type: %s", refType)
	}
}

// resolveRooms returns available rooms
func (r *DefaultObjectRefResolver) resolveRooms(ctx context.Context, filter map[string]interface{}) ([]*config.ObjectRefOption, error) {
	var rooms []facilities.Room
	query := r.db.NewSelect().
		Model(&rooms).
		ModelTableExpr(`facilities.rooms AS "room"`).
		Column("id", "name", "building").
		Where(`"room".deleted_at IS NULL`).
		Order("name ASC")

	// Apply filters
	if building, ok := filter["building"].(string); ok && building != "" {
		query = query.Where(`"room".building = ?`, building)
	}
	if category, ok := filter["category"].(string); ok && category != "" {
		query = query.Where(`"room".category = ?`, category)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, err
	}

	options := make([]*config.ObjectRefOption, len(rooms))
	for i, room := range rooms {
		name := room.Name
		if room.Building != "" {
			name = fmt.Sprintf("%s (%s)", room.Name, room.Building)
		}

		options[i] = &config.ObjectRefOption{
			ID:   room.ID,
			Name: name,
			Metadata: map[string]interface{}{
				"building": room.Building,
			},
		}
	}

	return options, nil
}

// resolveGroups returns available groups
func (r *DefaultObjectRefResolver) resolveGroups(ctx context.Context, filter map[string]interface{}) ([]*config.ObjectRefOption, error) {
	var groups []education.Group
	query := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`education.groups AS "group"`).
		Column("id", "name").
		Where(`"group".deleted_at IS NULL`).
		Order("name ASC")

	if err := query.Scan(ctx); err != nil {
		return nil, err
	}

	options := make([]*config.ObjectRefOption, len(groups))
	for i, group := range groups {
		options[i] = &config.ObjectRefOption{
			ID:   group.ID,
			Name: group.Name,
		}
	}

	return options, nil
}

// resolveStaff returns available staff members
func (r *DefaultObjectRefResolver) resolveStaff(ctx context.Context, filter map[string]interface{}) ([]*config.ObjectRefOption, error) {
	type staffResult struct {
		ID        int64  `bun:"id"`
		FirstName string `bun:"first_name"`
		LastName  string `bun:"last_name"`
		StaffType string `bun:"staff_type"`
	}

	var results []staffResult
	query := r.db.NewRaw(`
		SELECT s.id, p.first_name, p.last_name, s.staff_type
		FROM users.staff s
		INNER JOIN users.persons p ON p.id = s.person_id
		WHERE s.deleted_at IS NULL AND p.deleted_at IS NULL
		ORDER BY p.last_name, p.first_name
	`)

	if _, err := query.Exec(ctx, &results); err != nil {
		return nil, err
	}

	options := make([]*config.ObjectRefOption, len(results))
	for i, staff := range results {
		options[i] = &config.ObjectRefOption{
			ID:   staff.ID,
			Name: fmt.Sprintf("%s %s", staff.FirstName, staff.LastName),
			Metadata: map[string]interface{}{
				"staff_type": staff.StaffType,
			},
		}
	}

	return options, nil
}

// resolveTeachers returns available teachers
func (r *DefaultObjectRefResolver) resolveTeachers(ctx context.Context, filter map[string]interface{}) ([]*config.ObjectRefOption, error) {
	type teacherResult struct {
		ID           int64   `bun:"id"`
		FirstName    string  `bun:"first_name"`
		LastName     string  `bun:"last_name"`
		Abbreviation *string `bun:"abbreviation"`
	}

	var results []teacherResult
	query := r.db.NewRaw(`
		SELECT t.id, p.first_name, p.last_name, t.abbreviation
		FROM users.teachers t
		INNER JOIN users.staff s ON s.id = t.staff_id
		INNER JOIN users.persons p ON p.id = s.person_id
		WHERE t.deleted_at IS NULL AND s.deleted_at IS NULL AND p.deleted_at IS NULL
		ORDER BY p.last_name, p.first_name
	`)

	if _, err := query.Exec(ctx, &results); err != nil {
		return nil, err
	}

	options := make([]*config.ObjectRefOption, len(results))
	for i, teacher := range results {
		name := fmt.Sprintf("%s %s", teacher.FirstName, teacher.LastName)
		if teacher.Abbreviation != nil && *teacher.Abbreviation != "" {
			name = fmt.Sprintf("%s (%s)", name, *teacher.Abbreviation)
		}

		options[i] = &config.ObjectRefOption{
			ID:   teacher.ID,
			Name: name,
		}
	}

	return options, nil
}

// resolveStudents returns available students
func (r *DefaultObjectRefResolver) resolveStudents(ctx context.Context, filter map[string]interface{}) ([]*config.ObjectRefOption, error) {
	var students []users.Student
	query := r.db.NewSelect().
		Model(&students).
		ModelTableExpr(`users.students AS "student"`).
		Relation("Person").
		Where(`"student".deleted_at IS NULL`).
		Order(`"person".last_name`, `"person".first_name`)

	// Apply filters
	if groupID, ok := filter["group_id"].(float64); ok {
		query = query.
			Join(`INNER JOIN education.group_students gs ON gs.student_id = "student".id`).
			Where("gs.group_id = ?", int64(groupID))
	}

	if err := query.Scan(ctx); err != nil {
		return nil, err
	}

	options := make([]*config.ObjectRefOption, len(students))
	for i, student := range students {
		var name string
		if student.Person != nil {
			name = fmt.Sprintf("%s %s", student.Person.FirstName, student.Person.LastName)
		}

		options[i] = &config.ObjectRefOption{
			ID:   student.ID,
			Name: name,
		}
	}

	return options, nil
}

// resolveActivities returns available activities
func (r *DefaultObjectRefResolver) resolveActivities(ctx context.Context, filter map[string]interface{}) ([]*config.ObjectRefOption, error) {
	type activityResult struct {
		ID           int64  `bun:"id"`
		Name         string `bun:"name"`
		CategoryName string `bun:"category_name"`
	}

	var results []activityResult
	query := r.db.NewRaw(`
		SELECT a.id, a.name, COALESCE(c.name, '') as category_name
		FROM activities.activities a
		LEFT JOIN activities.activity_categories c ON c.id = a.category_id
		WHERE a.deleted_at IS NULL
		ORDER BY c.name, a.name
	`)

	if _, err := query.Exec(ctx, &results); err != nil {
		return nil, err
	}

	options := make([]*config.ObjectRefOption, len(results))
	for i, activity := range results {
		name := activity.Name
		if activity.CategoryName != "" {
			name = fmt.Sprintf("%s - %s", activity.CategoryName, activity.Name)
		}

		options[i] = &config.ObjectRefOption{
			ID:   activity.ID,
			Name: name,
		}
	}

	return options, nil
}

// resolveDevices returns available IoT devices
func (r *DefaultObjectRefResolver) resolveDevices(ctx context.Context, filter map[string]interface{}) ([]*config.ObjectRefOption, error) {
	type deviceResult struct {
		ID         int64   `bun:"id"`
		DeviceID   string  `bun:"device_id"`
		Name       *string `bun:"name"`
		DeviceType string  `bun:"device_type"`
	}

	var results []deviceResult
	query := r.db.NewRaw(`
		SELECT id, device_id, name, device_type
		FROM iot.devices
		WHERE deleted_at IS NULL
		ORDER BY COALESCE(name, device_id)
	`)

	if _, err := query.Exec(ctx, &results); err != nil {
		return nil, err
	}

	options := make([]*config.ObjectRefOption, len(results))
	for i, device := range results {
		name := device.DeviceID
		if device.Name != nil && *device.Name != "" {
			name = *device.Name
		}

		options[i] = &config.ObjectRefOption{
			ID:   device.ID,
			Name: name,
			Metadata: map[string]interface{}{
				"device_id":   device.DeviceID,
				"device_type": device.DeviceType,
			},
		}
	}

	return options, nil
}
