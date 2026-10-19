package owners

import (
	"context"
	"time"

	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	enrollmentProvenanceView "github.com/moto-nrw/project-phoenix/modules/timetableenrollmentprovenance/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/observability"
)

func NewCleanupTimetable(db transactionruntime.Database) (timetableModule.Capability, error) {
	students, err := peopleCompose.New(peopleCompose.Dependencies{StudentDirectory: studentDirectoryView.NewReader(), DB: db.Pool(), Observe: func(peopleCompose.Observation) {}, GuardianLinkRows: guardianLinkView.NewReader()})
	if err != nil {
		return nil, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return nil, err
	}
	return NewUnobservedTimetable(db, students, rooms)
}

func newTimetable(db transactionruntime.Database, students peopleModule.StudentQuery, rooms facilitiesModule.Query, membership schoolMembershipModule.Capability, observe func(timetableCompose.Observation)) (*timetableModule.Module, error) {
	return timetableCompose.New(timetableCompose.Dependencies{
		LockStaffAssignment: func(ctx context.Context, staffID int64) error {
			_, err := membership.FindStaffForMutation(ctx, staffID)
			return err
		},
		DB: db.Pool(), Students: timetableStudents(students), Rooms: timetableRooms(rooms), Sessions: NewPresenceFacts(db),
		Observe: observe, Provenance: enrollmentProvenanceView.NewReader(),
	})
}

// ComposeSchoolMembership composes the observed School Membership owner. A
// staff member is School Membership's membership row plus Workforce's
// employment profile (#2753); the membership owner composes the two.
func ComposeSchoolMembership(db transactionruntime.Database) (*schoolMembershipModule.Module, error) {
	dependencies, err := SchoolMembershipDependencies(db)
	if err != nil {
		return nil, err
	}
	return schoolMembershipCompose.New(dependencies)
}

// SchoolMembershipDependencies are the observed School Membership owner's
// dependencies, shared by the owner and its audited class-list
// administration.
func SchoolMembershipDependencies(db transactionruntime.Database) (schoolMembershipCompose.Dependencies, error) {
	pool := db.Pool()
	staffEmployment, err := workforceCompose.NewStaffEmployment(pool, func(observation workforceCompose.Observation) {
		observability.ObserveWorkforceOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, workforceModule.ErrorCode(observation.Err), observation.Err)
	})
	if err != nil {
		return schoolMembershipCompose.Dependencies{}, err
	}
	return schoolMembershipCompose.Dependencies{
		DB: pool,
		Observe: func(observation schoolMembershipCompose.Observation) {
			observability.ObserveSchoolMembershipOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolMembershipModule.ErrorCode(observation.Err), observation.Err)
		},
		Employment: MembershipStaffEmployment(staffEmployment),
		// Every counting membership write is checked against the school's
		// Kinderkontingent, which Organisation & Tenancy owns (#3567).
		ChildQuota: organizationCompose.NewChildQuotaLimits(),
	}, nil
}

// composeFeedback composes the observed Feedback owner over its settings.
func composeFeedback(db transactionruntime.Database, settings *feedbackCompose.Settings) (*feedbackModule.Module, error) {
	return feedbackCompose.New(feedbackCompose.Dependencies{
		DB:       db.Pool(),
		Settings: settings,
		Today:    feedbackModule.Today,
		Observe: func(observation feedbackCompose.Observation) {
			observability.ObserveFeedbackOperation(
				observation.Operation,
				observation.Duration,
				observation.Stats.Queries,
				observation.Stats.Rows,
				observation.Stats.StatementDuration,
				feedbackModule.ErrorCode(observation.Err),
				observation.Err,
			)
		},
	})
}

func ComposeCarePlan(db transactionruntime.Database, persons *peopleModule.Module, slots carePlanCompose.StatusSlotDirectory) (*carePlanModule.Module, error) {
	profiles, err := carePlanCompose.NewStudentProfiles(db.Pool(), studentDirectoryView.NewReader(), func(carePlanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	statusStudents, err := CarePlanStatusStudents(persons, profiles)
	if err != nil {
		return nil, err
	}
	return carePlanCompose.New(carePlanCompose.Dependencies{
		StudentDirectory: studentDirectoryView.NewReader(),
		DB:               db.Pool(), AmbientDB: carePlanCompose.TenantAmbientDatabase(db.Pool()),
		StatusStudents: statusStudents, StatusSlots: slots,
		People: carePlanStudentNames(func(ctx context.Context, ids []int64) ([]carePlanPorts.StudentName, error) {
			values, err := persons.ListStudentNamesByID(ctx, ids)
			if err != nil {
				return nil, err
			}
			result := make([]carePlanPorts.StudentName, 0, len(values))
			for _, value := range values {
				result = append(result, carePlanPorts.StudentName{StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName})
			}
			return result, nil
		}),
		StudentLock: persons.LockStudent, StudentNotFound: peopleModule.ErrStudentNotFound,
		Observe: func(observation carePlanCompose.Observation) {
			observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, carePlanModule.ErrorCode(observation.Err), observation.Err)
		},
	})
}

func mealPlanParticipantFinder(students StudentRootReads, now func() time.Time) mealplanCompose.ParticipantFinder {
	return func(ctx context.Context, date string) ([]mealplanCompose.ParticipantCandidate, error) {
		rows, err := students.OverlappingOnDate(ctx, date, now())
		if err != nil {
			return nil, err
		}
		candidates := make([]mealplanCompose.ParticipantCandidate, 0, len(rows))
		for _, row := range rows {
			candidates = append(candidates, mealplanCompose.ParticipantCandidate{
				StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass,
			})
		}
		return candidates, nil
	}
}
