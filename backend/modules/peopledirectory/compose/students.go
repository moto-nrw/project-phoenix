package compose

import (
	"context"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (e engine) ListStudentsByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Student, error) {
	values, err := e.students.ListByIDs(ctx, ids)
	return toPublicStudents(values), mapError(err)
}

func (e engine) ListStudentNamesByIDs(ctx context.Context, ids []int64) ([]peopledirectory.StudentName, error) {
	values, err := e.students.ListNamesByIDs(ctx, ids)
	result := make([]peopledirectory.StudentName, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentName(value))
	}
	return result, mapError(err)
}

func (e engine) ListStudentsAcrossTenantsByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Student, error) {
	values, err := e.students.ListAcrossTenantsByIDs(ctx, ids)
	return toPublicStudents(values), mapError(err)
}

func (e engine) ListStudentsByClasses(ctx context.Context, classes []string) ([]peopledirectory.Student, error) {
	values, err := e.students.ListByClasses(ctx, classes)
	return toPublicStudents(values), mapError(err)
}

func (e engine) ListStudentsByPersonIDs(ctx context.Context, personIDs []int64) ([]peopledirectory.Student, error) {
	values, err := e.students.ListByPersonIDs(ctx, personIDs)
	return toPublicStudents(values), mapError(err)
}

func (e engine) ListEnrolledStudents(ctx context.Context) ([]peopledirectory.Student, error) {
	values, err := e.students.ListEnrolled(ctx)
	return toPublicStudents(values), mapError(err)
}

func (e engine) ListSchoolClasses(ctx context.Context) ([]string, error) {
	values, err := e.students.ListClasses(ctx)
	return values, mapError(err)
}

func (e engine) ListStudentsWithStatusFlag(ctx context.Context, status string) ([]peopledirectory.Student, error) {
	values, err := e.students.ListByStatusFlag(ctx, status)
	return toPublicStudents(values), mapError(err)
}

func (e engine) LockStudent(ctx context.Context, id int64) error {
	return mapError(e.students.Lock(ctx, id))
}

func (e engine) PromoteStudents(ctx context.Context, ids []int64, fromClass, toClass string) (int64, error) {
	affected, err := e.students.Promote(ctx, ids, fromClass, toClass)
	return affected, mapError(err)
}

func (e engine) RevertStudentClass(ctx context.Context, id int64, fromClass, toClass string) (int64, error) {
	affected, err := e.students.RevertClass(ctx, id, fromClass, toClass)
	return affected, mapError(err)
}

func (e engine) GraduateStudentsByClasses(ctx context.Context, classes []string) (int64, error) {
	affected, err := e.students.GraduateByClasses(ctx, classes)
	return affected, mapError(err)
}

func (e engine) GraduateStudents(ctx context.Context, ids []int64) (int64, error) {
	affected, err := e.students.GraduateByIDs(ctx, ids)
	return affected, mapError(err)
}

func (e engine) ReactivateStudents(ctx context.Context, ids []int64, status string) ([]int64, error) {
	values, err := e.students.Reactivate(ctx, ids, status)
	return values, mapError(err)
}

func (e engine) ClearStudentStatusFlags(ctx context.Context, ids []int64, status string) (int64, error) {
	affected, err := e.students.ClearStatusFlags(ctx, ids, status)
	return affected, mapError(err)
}

func (e engine) SetFamilyProtection(ctx context.Context, input peopledirectory.SetFamilyProtection) (bool, error) {
	enabled, err := e.students.SetFamilyProtection(ctx, domain.FamilyProtectionChange{
		StudentID: input.StudentID, Enabled: input.Enabled,
		Reason: input.Reason, ActorAccountID: input.ActorAccountID,
	})
	return enabled, mapError(err)
}

func (e engine) ListStudentDirectory(
	ctx context.Context,
	filter peopledirectory.StudentDirectoryFilter,
) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListDirectory(ctx, toDomainStudentDirectoryFilter(filter))
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]peopledirectory.StudentRecord, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentRecord(value))
	}
	return result, nil
}

func (e engine) CountStudentDirectory(
	ctx context.Context,
	filter peopledirectory.StudentDirectoryFilter,
) (int, error) {
	total, err := e.students.CountDirectory(ctx, toDomainStudentDirectoryFilter(filter))
	return total, mapError(err)
}

func (e engine) ListStudentDirectoryIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.students.ListDirectoryIDs(ctx)
	return ids, mapError(err)
}

func (e engine) FindStudentRecordForMutation(ctx context.Context, studentID int64) (peopledirectory.StudentRecord, error) {
	record, err := e.students.FindRecord(ctx, studentID, "UPDATE")
	return peopledirectory.StudentRecord(record), mapError(err)
}

func (e engine) ListStudentRecordsByID(ctx context.Context, ids []int64) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListRecordsByIDs(ctx, ids)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]peopledirectory.StudentRecord, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentRecord(value))
	}
	return result, nil
}

func (e engine) LockStudentPhotoFeature(ctx context.Context) error {
	return mapPhotoError(e.studentPhotos.LockFeature(ctx))
}

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
		GuardianNameContains: filter.GuardianNameContains, KeepAlumni: filter.KeepAlumni,
		CareStatus: filter.CareStatus, CareStatusOn: filter.CareStatusOn,
		Page: filter.Page, PageSize: filter.PageSize,
	}
}

func toPublicStudents(values []domain.Student) []peopledirectory.Student {
	result := make([]peopledirectory.Student, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.Student{
			ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, TenantID: value.TenantID,
			PersonID: value.PersonID, SchoolClass: value.SchoolClass, GroupID: value.GroupID, Status: value.Status,
			EnrolledFrom: value.EnrolledFrom, EnrolledUntil: value.EnrolledUntil,
			Sick: value.Sick, SickSince: value.SickSince, Excused: value.Excused, ExcusedSince: value.ExcusedSince,
			PhotoPath: value.PhotoPath,
		})
	}
	return result
}

func (e engine) FindStudentRecord(ctx context.Context, studentID int64) (peopledirectory.StudentRecord, error) {
	record, err := e.students.FindRecord(ctx, studentID, "")
	return peopledirectory.StudentRecord(record), mapError(err)
}

func (e engine) ListStudentRecordsByPerson(ctx context.Context, personIDs []int64) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListRecordsByPersonIDs(ctx, personIDs)
	return toPublicStudentRecords(values), mapError(err)
}

func (e engine) ListStudentRecordsByGroup(ctx context.Context, groupIDs []int64) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListRecordsByGroups(ctx, groupIDs)
	return toPublicStudentRecords(values), mapError(err)
}

func (e engine) ListStudentRecordsByClass(ctx context.Context, classes []string) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListRecordsByClasses(ctx, classes)
	return toPublicStudentRecords(values), mapError(err)
}

func (e engine) ListStudentRecordsByGuardianContact(ctx context.Context, email, phone string) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListRecordsByGuardianContact(ctx, email, phone)
	return toPublicStudentRecords(values), mapError(err)
}

func (e engine) ListStudentRecordsDueForStatus(ctx context.Context, status, bound, asOf string) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListRecordsDueForStatus(ctx, status, bound, asOf)
	return toPublicStudentRecords(values), mapError(err)
}

func (e engine) LockStudentRecordsByID(ctx context.Context, ids []int64) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.LockRecordsByIDs(ctx, ids)
	return toPublicStudentRecords(values), mapError(err)
}

func (e engine) CountStudentsByGroup(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	counts, err := e.students.CountByGroups(ctx, groupIDs)
	return counts, mapError(err)
}

func (e engine) ListEnrolledStudentIDsByNameAndBirthday(
	ctx context.Context,
	tenantID int64,
	firstName, lastName, birthday string,
) ([]int64, error) {
	ids, err := e.students.ListEnrolledIDsByNameAndBirthday(ctx, tenantID, firstName, lastName, birthday)
	return ids, mapError(err)
}

func toPublicStudentRecords(values []domain.StudentRecord) []peopledirectory.StudentRecord {
	result := make([]peopledirectory.StudentRecord, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentRecord(value))
	}
	return result
}

func (e engine) ListAllStudentIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.students.ListAllIDs(ctx)
	return ids, mapError(err)
}

func (e engine) FindStudentRecordForMutationNoWait(
	ctx context.Context,
	studentID int64,
) (peopledirectory.StudentRecord, error) {
	record, err := e.students.FindRecord(ctx, studentID, "UPDATE NOWAIT")
	return peopledirectory.StudentRecord(record), mapError(err)
}
