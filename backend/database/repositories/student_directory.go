package repositories

import (
	"context"
	"errors"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// This file is the translation seam of the staff student directory (#3349):
// People Directory owns the filtered, paginated read over users.students, and
// the retained handlers still carry users.Student rows. It holds no filter
// rules of its own — which columns a caller may narrow by is the owner's
// decision, expressed by peopledirectory.StudentDirectoryFilter.

// StudentDirectoryCapability is the owner surface this seam translates to.
type StudentDirectoryCapability interface {
	peopleModule.StudentDirectoryQuery
	peopleModule.StudentDirectoryCommand
	// LockEnrollmentClassWrites is the owner's shared class-writes gate; the
	// retained update path takes it for the same reason the enrollment flows do.
	LockEnrollmentClassWrites(ctx context.Context) error
}

// StudentDirectory adapts the owner's directory read to the retained
// model-typed contract (services/users.StudentDirectoryReader, satisfied
// structurally so this seam does not depend on it).
type StudentDirectory struct{ directory StudentDirectoryCapability }

// NewStudentDirectory adapts the owner's directory read to the retained
// model-typed contract.
func NewStudentDirectory(directory StudentDirectoryCapability) *StudentDirectory {
	return &StudentDirectory{directory: directory}
}

func (d *StudentDirectory) ListStudents(
	ctx context.Context,
	filter userModels.StudentDirectoryFilter,
) ([]*userModels.Student, error) {
	records, err := d.directory.ListStudentDirectory(ctx, toOwnerStudentDirectoryFilter(filter))
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	result := make([]*userModels.Student, 0, len(records))
	for _, record := range records {
		result = append(result, studentRecordToModel(record))
	}
	return result, nil
}

func (d *StudentDirectory) CountStudents(ctx context.Context, filter userModels.StudentDirectoryFilter) (int, error) {
	return d.directory.CountStudentDirectory(ctx, toOwnerStudentDirectoryFilter(filter))
}

func (d *StudentDirectory) ListStudentIDs(ctx context.Context) ([]int64, error) {
	return d.directory.ListStudentDirectoryIDs(ctx)
}

func toOwnerStudentDirectoryFilter(filter userModels.StudentDirectoryFilter) peopleModule.StudentDirectoryFilter {
	owner := peopleModule.StudentDirectoryFilter{
		IDs: filter.IDs, SchoolClasses: filter.SchoolClasses, GradeLevels: filter.GradeLevels,
		KeepAlumni: filter.KeepAlumni,
		CareStatus: filter.CareStatus, Page: filter.Page, PageSize: filter.PageSize,
	}
	if filter.CareStatusOn != "" {
		owner.CareStatusOn = filter.CareStatusOn.String()
	}
	return owner
}

// studentRecordToModel rebuilds the retained row from the owner's record. The
// calendar dates travel as YYYY-MM-DD, so a malformed one would be a bug in
// the owner rather than input: it becomes an unset date instead of a wrong day.
func studentRecordToModel(record peopleModule.StudentRecord) *userModels.Student {
	student := &userModels.Student{
		PersonID: record.PersonID, SchoolClass: record.SchoolClass, GroupID: record.GroupID,
		Status: userModels.StudentStatus(record.Status),

		AddressStreet:     record.AddressStreet,
		AddressCity:       record.AddressCity,
		AddressPostalCode: record.AddressPostalCode,

		ExtraInfo:       record.ExtraInfo,
		SupervisorNotes: record.SupervisorNotes,
		HealthInfo:      record.HealthInfo,
		PickupStatus:    record.PickupStatus,

		DepartureDays:          record.DepartureDays,
		AllowedDepartureModes:  record.AllowedDepartureModes,
		PickupDays:             record.PickupDays,
		BusDays:                record.BusDays,
		DepartureCompanionNote: record.DepartureCompanionNote,

		Sick:         record.Sick,
		SickSince:    record.SickSince,
		Excused:      record.Excused,
		ExcusedSince: record.ExcusedSince,

		PhotoPath:           record.PhotoPath,
		PhotoConsentGivenAt: record.PhotoConsentGivenAt,
		PhotoConsentGivenBy: record.PhotoConsentGivenBy,

		AGBAcceptedAt:            record.AGBAcceptedAt,
		DataProcessingAcceptedAt: record.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   record.EmailContactAcceptedAt,
	}
	student.ID, student.CreatedAt, student.UpdatedAt = record.ID, record.CreatedAt, record.UpdatedAt
	student.SetTenantID(record.TenantID)
	student.EnrolledFrom = userModels.OptionalCalendarDate(record.EnrolledFrom)
	student.EnrolledUntil = userModels.OptionalCalendarDate(record.EnrolledUntil)
	hydrateDeparturePlan(student)
	return student
}

// hydrateDeparturePlan resolves the stored departure projections into the one
// effective plan and records it as the update baseline. The precedence is the
// owner's (departure.Plan.Effective); the snapshot is what lets a later Update
// tell a plan the caller intentionally changed from one that merely rode along
// on this read — without it, a caller that never touches the plan re-persists
// its copy over a concurrent companion edit (#1694).
func hydrateDeparturePlan(student *userModels.Student) {
	effective := userModels.DeparturePlan{
		AllowedDepartureModes: student.AllowedDepartureModes,
		DepartureDays:         student.DepartureDays,
		BusDays:               student.BusDays,
		PickupDays:            student.PickupDays,
	}.Effective()
	student.AllowedDepartureModes = effective.AllowedDepartureModes
	student.DepartureDays = effective.DepartureDays
	student.BusDays = effective.BusDays
	student.PickupDays = effective.PickupDays
	student.SnapshotDeparturePlan()
}

// translateStudentDirectoryError restates a child the owner would not hand
// over in the vocabulary the retained contract uses. A missing row and an
// unusable id collapse to the same answer on purpose: both mean "no such child
// of this tenant", which is what the retained repository reported for either,
// and what the handlers above render as 404 rather than 500.
func translateStudentDirectoryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, peopleModule.ErrStudentNotFound) || errors.Is(err, peopleModule.ErrInvalidStudent) {
		return userModels.ErrStudentRowMissing
	}
	return err
}

func (d *StudentDirectory) GetStudentsByID(ctx context.Context, ids []int64) ([]*userModels.Student, error) {
	records, err := d.directory.ListStudentRecordsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*userModels.Student, 0, len(records))
	for _, record := range records {
		result = append(result, studentRecordToModel(record))
	}
	return result, nil
}

func (d *StudentDirectory) LockStudent(ctx context.Context, id int64) (*userModels.Student, error) {
	record, err := d.directory.FindStudentRecordForMutation(ctx, id)
	if err != nil {
		return nil, translateStudentDirectoryError(err)
	}
	return studentRecordToModel(record), nil
}

func (d *StudentDirectory) LockPhotoFeature(ctx context.Context) error {
	return d.directory.LockStudentPhotoFeature(ctx)
}

func (d *StudentDirectory) LockClassWritesShared(ctx context.Context) error {
	return d.directory.LockEnrollmentClassWrites(ctx)
}
