package owners

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// NewEnrollmentReports composes Enrollment's reports over the root's reads.
// A missing read stays unbound, so the report that needs it answers "not
// configured" instead of dereferencing nothing.
func NewEnrollmentReports(sources EnrollmentReportSources) enrollmentOwner.Reports {
	if sources.Settings == nil {
		panic("enrollment reports: settings resolver is required")
	}
	deps := enrollmentCompose.ReportDependencies{
		Requests:          sources.Owner,
		Children:          sources.Owner,
		Guardians:         sources.Owner,
		Schemas:           sources.Owner,
		Phases:            sources.Owner,
		Offerings:         sources.Offerings,
		Companions:        sources.Companions,
		ClassListEntries:  sources.ClassListEntries,
		PickupSchedules:   sources.PickupSchedules,
		CareParticipation: sources.CareParticipation,
		Settings:          enrollmentReportSettings{settings: sources.Settings},
	}
	if sources.AccessLog != nil {
		deps.AccessLog = enrollmentExportAccessLog{repo: sources.AccessLog}
	}
	if sources.Students != nil {
		deps.Students = enrollmentRosterStudents{repo: sources.Students}
	}
	if sources.Persons != nil {
		deps.Persons = enrollmentRosterPersons{repo: sources.Persons}
	}
	if sources.Groups != nil {
		deps.Groups = sources.Groups
	}
	if sources.StudentGuardians != nil {
		deps.GuardianContacts = enrollmentRosterGuardianContacts{repo: sources.StudentGuardians}
	}
	return enrollmentCompose.NewReports(deps)
}

type enrollmentRosterStudents struct{ repo enrollmentRosterStudentRows }

func (b enrollmentRosterStudents) ListClassRoster(ctx context.Context, schoolClass string) ([]*enrollmentCompose.RosterStudent, error) {
	students, err := b.repo.ListClassRoster(ctx, schoolClass)
	if err != nil {
		return nil, err
	}
	out := make([]*enrollmentCompose.RosterStudent, 0, len(students))
	for _, student := range students {
		if student == nil {
			continue
		}
		out = append(out, &enrollmentCompose.RosterStudent{
			ID: student.ID, PersonID: student.PersonID, SchoolClass: student.SchoolClass, GroupID: student.GroupID,
			AllowedDepartureModes: student.AllowedDepartureModes, DepartureDays: student.DepartureDays,
			DepartureCompanionNote: student.DepartureCompanionNote,
		})
	}
	return out, nil
}

type enrollmentRosterPersons struct{ repo enrollmentRosterPersonRows }

func (b enrollmentRosterPersons) PersonsByID(ctx context.Context, personIDs []int64) (map[int64]*enrollmentCompose.RosterPerson, error) {
	persons, err := b.repo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*enrollmentCompose.RosterPerson, len(persons))
	for id, person := range persons {
		if person != nil {
			out[id] = &enrollmentCompose.RosterPerson{FirstName: person.FirstName, LastName: person.LastName}
		}
	}
	return out, nil
}

type enrollmentRosterGuardianContacts struct {
	repo enrollmentGuardianContactRows
}

func (b enrollmentRosterGuardianContacts) GuardianContacts(ctx context.Context, studentIDs []int64) ([]enrollmentCompose.GuardianContactRow, error) {
	rows, err := b.repo.ListEmergencyContactRows(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	out := make([]enrollmentCompose.GuardianContactRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, enrollmentCompose.GuardianContactRow{
			StudentID: row.StudentID, GuardianProfileID: row.GuardianProfileID,
			FirstName: StringValue(row.FirstName), LastName: StringValue(row.LastName),
			Email: StringValue(row.Email), PhoneNumber: StringValue(row.PhoneNumber),
		})
	}
	return out, nil
}

// enrollmentReportSettings resolves enrollment.care_offerings_enabled through Settings Platform.
type enrollmentReportSettings struct {
	settings interface {
		ResolveBool(ctx context.Context, key string) (bool, error)
	}
}

func (s enrollmentReportSettings) CareOfferingsEnabled(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentCareOfferingsEnabled)
}

// enrollmentExportAccessLog appends the enrollment export rows to
// audit.data_access_log.
type enrollmentExportAccessLog struct {
	repo enrollmentAccessLogAppender
}

func (l enrollmentExportAccessLog) RecordPhaseExport(ctx context.Context, entry enrollmentCompose.ExportAccess) error {
	return l.repo.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: entry.ActorAccountID,
		ActorRole:      entry.ActorRole,
		ResourceType:   auditModels.ResourceTypeEnrollmentPhaseExport,
		RangeStart:     entry.RangeStart,
		RangeEnd:       entry.RangeEnd,
		AccessedAt:     entry.AccessedAt,
		Metadata:       entry.Metadata,
	})
}

func (l enrollmentExportAccessLog) RecordStudentExport(ctx context.Context, studentID int64, entry enrollmentCompose.ExportAccess) error {
	return l.repo.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: entry.ActorAccountID,
		ActorRole:      entry.ActorRole,
		ResourceType:   auditModels.ResourceTypeEnrollmentStudentExport,
		StudentID:      &studentID,
		RangeStart:     entry.RangeStart,
		RangeEnd:       entry.RangeEnd,
		AccessedAt:     entry.AccessedAt,
		Metadata:       entry.Metadata,
	})
}

// EnrollmentReportOwnerReads are the Enrollment owner reads of the reports.
type EnrollmentReportOwnerReads interface {
	enrollmentCompose.ReportRequests
	enrollmentCompose.ReportChildren
	enrollmentCompose.ReportGuardians
	enrollmentCompose.ReportSchemas
	enrollmentCompose.ReportPhases
}

// EnrollmentReportSources are the reads the root binds the reports to.
type EnrollmentReportSources struct {
	Owner             EnrollmentReportOwnerReads
	Offerings         enrollmentCompose.ReportOfferings
	AccessLog         enrollmentAccessLogAppender
	Students          enrollmentRosterStudentRows
	Persons           enrollmentRosterPersonRows
	Groups            enrollmentCompose.RosterGroups
	StudentGuardians  enrollmentGuardianContactRows
	Companions        enrollmentCompose.RosterCompanions
	ClassListEntries  enrollmentCompose.ClassListEntries
	PickupSchedules   enrollmentCompose.PickupSchedules
	CareParticipation enrollmentCompose.CareParticipation
	Settings          interface {
		ResolveBool(ctx context.Context, key string) (bool, error)
	}
}
