package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	feedbackAPI "github.com/moto-nrw/project-phoenix/modules/feedback/http"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	mealplanAPI "github.com/moto-nrw/project-phoenix/modules/mealplan/http"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
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

// recordHTTPRuntimeEvent turns one runtime event into a metric and, for
// failures, one ERROR record carrying method, route, path and status. A
// rolled-back transaction is only a failure when the client got a 5xx: the
// tenant middleware and service-owned transactions also roll back behind
// 401/404/410 responses, and those must not feed the error-spike alert
// (#2953). The slog-chi request line already records every 4xx at WARN.
type httpRuntimeObservation = apiCommon.TenantRuntimeObservation

func recordHTTPRuntimeEvent(tracer *observability.Tracer, observation httpRuntimeObservation) {
	event, r, status := observation.Event, observation.Request, observation.Status
	observability.RecordUnitOfWorkEvent(
		"http",
		string(event.Kind),
		string(event.Result),
		event.Duration,
		event.Retries,
	)
	ctx := r.Context()
	attrs := []slog.Attr{
		slog.String("method", r.Method),
		slog.String("route", observation.Route),
		slog.String("path", customMiddleware.RedactFeedToken(r.URL.Path)),
		slog.Int("status", status),
	}
	switch {
	case event.Kind == apiCommon.TenantRuntimeMissingTenant:
		tracer.Failure(ctx, "http", string(event.Kind), "missing_tenant", event.Err, attrs...)
	case event.Kind == apiCommon.TenantRuntimeTransaction && event.Err != nil && status >= http.StatusInternalServerError:
		attrs = append(attrs, slog.String("result", string(event.Result)))
		tracer.Failure(ctx, "http", string(event.Kind), "transaction_failure", event.Err, attrs...)
	case event.Kind == apiCommon.TenantRuntimeResponseWrite && event.Err != nil:
		tracer.Failure(ctx, "http", string(event.Kind), "response_write_failure", event.Err, attrs...)
	}
}

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

// composeSchoolMembership composes the observed School Membership owner. A
// staff member is School Membership's membership row plus Workforce's
// employment profile (#2753); the membership owner composes the two.
func composeSchoolMembership(db transactionruntime.Database) (*schoolMembershipModule.Module, error) {
	dependencies, err := schoolMembershipDependencies(db)
	if err != nil {
		return nil, err
	}
	return schoolMembershipCompose.New(dependencies)
}

// schoolMembershipDependencies are the observed School Membership owner's
// dependencies, shared by the owner and its audited class-list
// administration.
func schoolMembershipDependencies(db transactionruntime.Database) (schoolMembershipCompose.Dependencies, error) {
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

// observeCommunication records one Communication operation.
func observeCommunication(observation communicationCompose.Observation) {
	observability.ObserveCommunicationOperation(
		observation.Operation, observation.Duration, int64(observation.Stats.Queries), observation.Stats.Rows,
		int64(observation.Stats.DuplicatePreventionConflicts), observation.Stats.StatementDuration,
		communicationModule.ErrorCode(observation.Err), observation.Err,
	)
}

func observeFileStorage(observation filestorageCompose.Observation) {
	observability.ObserveFileStorageOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, filestorageModule.ErrorCode(observation.Err), observation.Err)
}

func observeDataImport(observation importModels.ImportObservation) {
	observability.ObserveDataImport(observation.Entity, observation.DryRun, observation.Rows, observation.Accepted, observation.Rejected, observation.Created, observation.Updated, observation.Duration)
	if observation.DryRun {
		return
	}
	observability.ObserveDataImportRuntime(observation.Entity, observation.BatchesCommitted, observation.BatchesRetried, observation.CheckpointLag, observation.Deadlocks, observation.PoolWait, observation.LockWait)
	for _, command := range observation.Commands {
		observability.ObserveDataImportCommand(observation.Entity, command.Owner, command.Operation, command.Duration, command.Failed)
	}
}

func composeCarePlan(db transactionruntime.Database, persons *peopleModule.Module, slots carePlanCompose.StatusSlotDirectory) (*carePlanModule.Module, error) {
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

var mealPlanErrorRules = []apiCommon.ErrorRule{
	{Target: mealplanModule.ErrDisabled, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorForbiddenWithCode, errors.New(mealplanModule.ErrDisabled.Detail), apiCommon.CodeMealsPlanDisabled)},
	{Target: mealplanModule.ErrInvalidMealDate, Render: apiCommon.FixedRenderer(apiCommon.ErrorInvalidRequest, errors.New(mealplanModule.ErrInvalidMealDate.Detail))},
	{Target: mealplanModule.ErrInvalidDishes, Render: apiCommon.ErrorInvalidRequest},
	{Target: mealplanModule.ErrRegistrationDisabled, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorForbiddenWithCode, errors.New(mealplanModule.ErrRegistrationDisabled.Detail), apiCommon.CodeMealsRegistrationDisabled)},
	{Target: mealplanModule.ErrInvalidParticipation, Render: apiCommon.ErrorInvalidRequest},
	{Target: mealplanModule.ErrParticipationCutoff, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorConflictWithCode, errors.New(mealplanModule.ErrParticipationCutoff.Detail), apiCommon.CodeMealsParticipationCutoffPassed)},
}

func renderMealPlanFailure(w http.ResponseWriter, r *http.Request, err error, internalMessage string) {
	renderer := apiCommon.RulesRenderer(mealPlanErrorRules, apiCommon.ErrorInternalServerRenderer(internalMessage))
	apiCommon.RenderError(w, r, renderer(err))
}

func newMealPlanResource(module *mealplanModule.Module, renderer SimpleListRenderer) *mealplanAPI.Resource {
	return mealplanAPI.NewResource(module, mealplanAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, mealplanAPI.Middleware)) {
			ProtectedTenantRoutes(router, register)
		},
		Permission: func(access mealplanAPI.Access) mealplanAPI.Middleware {
			return requireMealPlanAccess(access)
		},
		Success: apiCommon.Respond,
		InvalidRequest: func(w http.ResponseWriter, r *http.Request, err error) {
			apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
		},
		ModuleFailure: func(w http.ResponseWriter, r *http.Request, err error, internalMessage string) {
			renderMealPlanFailure(w, r, err, internalMessage)
		},
		ExportDailyList: func(list mealplanModule.DailyList, format string) (mealplanAPI.ExportFile, error) {
			return renderMealParticipationExport(renderer, list, format)
		},
	})
}

func renderMealParticipationExport(renderer SimpleListRenderer, list mealplanModule.DailyList, format string) (mealplanAPI.ExportFile, error) {
	export, err := list.Export()
	if err != nil {
		return mealplanAPI.ExportFile{}, err
	}
	columns := make([]SimpleListColumn, 0, len(export.Columns))
	for _, column := range export.Columns {
		columns = append(columns, SimpleListColumn{ID: column.ID, Label: column.Label})
	}
	file, err := renderer(SimpleListDocument{
		Title: export.Title, Subtitle: export.Subtitle, GeneratedAt: time.Now(), Filters: export.Filters,
		Columns: columns, Rows: export.Lines, Footer: export.Footer,
	}, format, export.Filename)
	if err != nil {
		return mealplanAPI.ExportFile{}, err
	}
	return mealplanAPI.ExportFile{Data: file.Data, ContentType: file.ContentType, Filename: file.Filename}, nil
}

func newUsersResource(module peopleModule.Capability, accountEmails func(context.Context, []int64) (map[int64]string, error), tagExists func(context.Context, string) (bool, error)) *usersAPI.Resource {
	return usersAPI.NewResource(module, usersAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, usersAPI.Middleware)) {
			ProtectedTenantRoutes(router, register)
		},
		Permission: func(permission string) usersAPI.Middleware {
			return apiCommon.RequiresPermission(permission)
		},
		ParsePagination: apiCommon.ParsePagination,
		Success:         apiCommon.Respond,
		SuccessPaginated: func(w http.ResponseWriter, r *http.Request, status int, data any, pagination usersAPI.Pagination, message string) {
			apiCommon.RespondPaginated(w, r, status, data, apiCommon.PaginationParams{Page: pagination.Page, PageSize: pagination.PageSize, Total: pagination.Total}, message)
		},
		NoContent: apiCommon.RespondNoContent,
		Failure: func(w http.ResponseWriter, r *http.Request, kind usersAPI.FailureKind, err error) {
			switch kind {
			case usersAPI.FailureInvalidRequest:
				apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
			case usersAPI.FailureNotFound:
				apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
			case usersAPI.FailureConflict:
				apiCommon.RenderError(w, r, apiCommon.ErrorConflict(err))
			default:
				apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap("Internal server error", err))
			}
		},
		AccountEmails: accountEmails,
		TagExists:     tagExists,
		ObserveResponse: func(status int, code string) {
			observability.ObservePeopleDirectoryHTTPResponse(status, code)
		},
	})
}

func newFeedbackResource(module *feedbackModule.Module) *feedbackAPI.Resource {
	return feedbackAPI.NewResource(module, feedbackAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, feedbackAPI.Middleware)) {
			ProtectedTenantRoutes(router, register)
		},
		Permission: func(permission string) feedbackAPI.Middleware {
			return apiCommon.RequiresPermission(permission)
		},
		Success: apiCommon.Respond,
		Failure: func(w http.ResponseWriter, r *http.Request, failure feedbackAPI.Failure) {
			response := &apiCommon.ErrResponse{Err: failure.Err, HTTPStatusCode: failure.Status, Detail: failure.Err.Error()}
			if errors.Is(failure.Err, feedbackAPI.ErrFeatureDisabled) {
				response.Code = apiCommon.CodeFeedbackDisabled
			}
			apiCommon.RenderError(w, r, response)
		},
		ObserveResponse: func(status int, code string) {
			observability.ObserveFeedbackHTTPResponse("staff", status, code)
		},
	})
}
