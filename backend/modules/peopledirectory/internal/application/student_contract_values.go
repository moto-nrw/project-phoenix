package application

import (
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// toDomainStudentDirectoryFilter renders the grade levels the SQL compares
// against: the column holds free text, so the match is on the decimal spelling
// of each level.
func toDomainStudentDirectoryFilter(filter peopledirectory.StudentDirectoryFilter) domain.StudentDirectoryFilter {
	levels := make([]string, 0, len(filter.GradeLevels))
	for _, level := range filter.GradeLevels {
		levels = append(levels, strconv.Itoa(level))
	}
	return domain.StudentDirectoryFilter{
		IDs: filter.IDs, SchoolClasses: filter.SchoolClasses, GradeLevels: levels,
		KeepAlumni: filter.KeepAlumni,
		CareStatus: filter.CareStatus, CareStatusOn: filter.CareStatusOn,
		Page: filter.Page, PageSize: filter.PageSize,
	}
}

func nonNilStudents(values []peopledirectory.Student) []peopledirectory.Student {
	if values == nil {
		return []peopledirectory.Student{}
	}
	return values
}

func toPublicStudentRecords(values []domain.StudentRecord) []peopledirectory.StudentRecord {
	result := make([]peopledirectory.StudentRecord, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentRecord(value))
	}
	return result
}

func toPublicRoster(values []domain.StudentRosterEntry) []peopledirectory.StudentRosterEntry {
	result := make([]peopledirectory.StudentRosterEntry, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentRosterEntry{
			Record:    peopledirectory.StudentStateFromRecord(peopledirectory.StudentRecord(value.Record)),
			FirstName: value.FirstName, LastName: value.LastName,
			TagID: value.TagID, AccountID: value.AccountID,
		})
	}
	return result
}
