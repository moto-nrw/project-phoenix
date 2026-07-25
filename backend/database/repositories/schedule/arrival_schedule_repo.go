package schedule

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const (
	tableArrivalSchedules  = "schedule.student_arrival_schedules"
	tableArrivalExceptions = "schedule.student_arrival_exceptions"
	tableArrivalNotes      = "schedule.student_arrival_notes"
)

var (
	arrivalScheduleConfig = effectiveTimeRepositoryConfig{
		table:      tableArrivalSchedules,
		alias:      "student_arrival_schedule",
		entityName: "StudentArrivalSchedule",
	}
	arrivalExceptionConfig = effectiveTimeRepositoryConfig{
		table:      tableArrivalExceptions,
		alias:      "student_arrival_exception",
		entityName: "StudentArrivalException",
	}
	arrivalNoteConfig = effectiveTimeRepositoryConfig{
		table:      tableArrivalNotes,
		alias:      "student_arrival_note",
		entityName: "StudentArrivalNote",
	}
)

type StudentArrivalScheduleRepository struct {
	*studentScheduleRepository[*schedule.StudentArrivalSchedule]
}

func NewStudentArrivalScheduleRepository(db *bun.DB) schedule.StudentArrivalScheduleRepository {
	return &StudentArrivalScheduleRepository{
		studentScheduleRepository: newStudentScheduleRepository[*schedule.StudentArrivalSchedule](
			db,
			arrivalScheduleConfig,
			"expected_arrival",
			"upsert arrival schedule",
			errors.New("arrival schedule cannot be nil"),
		),
	}
}

type StudentArrivalExceptionRepository struct {
	*studentExceptionRepository[*schedule.StudentArrivalException]
}

func NewStudentArrivalExceptionRepository(db *bun.DB) schedule.StudentArrivalExceptionRepository {
	return &StudentArrivalExceptionRepository{
		studentExceptionRepository: newStudentExceptionRepository[*schedule.StudentArrivalException](
			db,
			arrivalExceptionConfig,
		),
	}
}

type StudentArrivalNoteRepository struct {
	*studentNoteRepository[*schedule.StudentArrivalNote]
}

func NewStudentArrivalNoteRepository(db *bun.DB) schedule.StudentArrivalNoteRepository {
	return &StudentArrivalNoteRepository{
		studentNoteRepository: newStudentNoteRepository[*schedule.StudentArrivalNote](
			db,
			arrivalNoteConfig,
		),
	}
}
