package schedule

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const (
	tablePickupSchedules  = "schedule.student_pickup_schedules"
	tablePickupExceptions = "schedule.student_pickup_exceptions"
	tablePickupNotes      = "schedule.student_pickup_notes"
)

var (
	pickupScheduleConfig = effectiveTimeRepositoryConfig{
		table:      tablePickupSchedules,
		alias:      "student_pickup_schedule",
		entityName: "StudentPickupSchedule",
	}
	pickupExceptionConfig = effectiveTimeRepositoryConfig{
		table:      tablePickupExceptions,
		alias:      "student_pickup_exception",
		entityName: "StudentPickupException",
	}
	pickupNoteConfig = effectiveTimeRepositoryConfig{
		table:      tablePickupNotes,
		alias:      "student_pickup_note",
		entityName: "StudentPickupNote",
	}
)

type StudentPickupScheduleRepository struct {
	*studentScheduleRepository[*schedule.StudentPickupSchedule]
}

func NewStudentPickupScheduleRepository(db *bun.DB) schedule.StudentPickupScheduleRepository {
	repo := newStudentScheduleRepository[*schedule.StudentPickupSchedule](
		db,
		pickupScheduleConfig,
		"pickup_time",
		"upsert schedule",
		errors.New("schedule cannot be nil"),
	)
	// Provenance travels with every upsert: a manual save (source staff)
	// reclaims an offering-sourced row, a reconciler save stamps its
	// offering (#2290).
	repo.extraUpsertSets = []string{
		"source = EXCLUDED.source",
		"care_offering_id = EXCLUDED.care_offering_id",
	}
	return &StudentPickupScheduleRepository{
		studentScheduleRepository: repo,
	}
}

type StudentPickupExceptionRepository struct {
	*studentExceptionRepository[*schedule.StudentPickupException]
}

func NewStudentPickupExceptionRepository(db *bun.DB) schedule.StudentPickupExceptionRepository {
	return &StudentPickupExceptionRepository{
		studentExceptionRepository: newStudentExceptionRepository[*schedule.StudentPickupException](
			db,
			pickupExceptionConfig,
		),
	}
}

type StudentPickupNoteRepository struct {
	*studentNoteRepository[*schedule.StudentPickupNote]
}

func NewStudentPickupNoteRepository(db *bun.DB) schedule.StudentPickupNoteRepository {
	return &StudentPickupNoteRepository{
		studentNoteRepository: newStudentNoteRepository[*schedule.StudentPickupNote](
			db,
			pickupNoteConfig,
		),
	}
}
