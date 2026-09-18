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
		GuardianNameContains: filter.GuardianNameContains, KeepAlumni: filter.KeepAlumni,
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
		Status:          userModels.StudentStatus(record.Status),
		GuardianName:    record.GuardianName,
		GuardianContact: record.GuardianContact,
		GuardianEmail:   record.GuardianEmail,
		GuardianPhone:   record.GuardianPhone,

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

// hydrateDeparturePlan resolves the three stored departure projections into the
// one effective plan and records it as the update baseline, exactly as the
// repository's own hydration does.
//
// Precedence: allowed_departure_modes wins, then departure_days, then the
// legacy bus/pickup maps — an empty departure_days cannot distinguish "walks
// alone every day" from "not backfilled yet", and a row written straight to
// bus_days would otherwise read as no plan at all.
//
// The snapshot is what lets a later Update tell a plan the caller intentionally
// changed from one that merely rode along on this read; without it, a caller
// that never touches the plan re-persists its copy over a concurrent companion
// edit (models/users.Student.DepartureBaseline, #1694).
func hydrateDeparturePlan(student *userModels.Student) {
	if allowed := student.AllowedDepartureModes.Normalize(); allowed.HasAny() {
		student.AllowedDepartureModes = allowed
		student.DepartureDays = allowed.DepartureDays()
		student.BusDays = allowed.BusDays()
		student.PickupDays = allowed.PickupDays()
		student.SnapshotDeparturePlan()
		return
	}
	if departure := student.DepartureDays.Normalize(); departure.HasAny() {
		student.DepartureDays = departure
		student.AllowedDepartureModes = userModels.AllowedDepartureModesFromDeparture(departure)
		student.BusDays = departure.BusDays()
		student.PickupDays = departure.PickupDays()
		student.SnapshotDeparturePlan()
		return
	}
	student.BusDays = student.BusDays.Normalize()
	student.PickupDays = student.PickupDays.Normalize()
	student.DepartureDays = userModels.DepartureDaysFromLegacy(student.BusDays, student.PickupDays)
	student.AllowedDepartureModes = userModels.AllowedDepartureModesFromLegacy(student.BusDays, student.PickupDays)
	student.SnapshotDeparturePlan()
}

func (d *StudentDirectory) GetStudent(ctx context.Context, id int64) (*userModels.Student, error) {
	record, err := d.directory.FindStudentRecord(ctx, id)
	if err != nil {
		return nil, translateStudentDirectoryError(err)
	}
	return studentRecordToModel(record), nil
}

// translateStudentDirectoryError restates a missing child in the vocabulary
// the retained contract uses. The owner has its own not-found sentinel, which
// this package's consumers cannot reference; the service above turns the
// shared one into the error shape its callers branch on.
func translateStudentDirectoryError(err error) error {
	if !errors.Is(err, peopleModule.ErrStudentNotFound) {
		return err
	}
	return userModels.ErrStudentRowMissing
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
