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
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackAPI "github.com/moto-nrw/project-phoenix/modules/feedback/http"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanAPI "github.com/moto-nrw/project-phoenix/modules/mealplan/http"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	usersAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
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

var mealPlanErrorRules = []apiCommon.ErrorRule{
	{Target: mealplanModule.ErrDisabled, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorForbiddenWithCode, errors.New(mealplanModule.ErrDisabled.Detail), apiCommon.CodeMealsPlanDisabled)},
	{Target: mealplanModule.ErrInvalidMealDate, Render: apiCommon.FixedRenderer(apiCommon.ErrorInvalidRequest, errors.New(mealplanModule.ErrInvalidMealDate.Detail))},
	{Target: mealplanModule.ErrInvalidDishes, Render: apiCommon.ErrorInvalidRequest},
	{Target: mealplanModule.ErrInvalidAllergens, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorInvalidRequestWithCode, errors.New(mealplanModule.ErrInvalidAllergens.Detail), apiCommon.CodeMealsInvalidAllergens)},
	{Target: mealplanModule.ErrRegistrationDisabled, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorForbiddenWithCode, errors.New(mealplanModule.ErrRegistrationDisabled.Detail), apiCommon.CodeMealsRegistrationDisabled)},
	{Target: mealplanModule.ErrInvalidParticipation, Render: apiCommon.ErrorInvalidRequest},
	{Target: mealplanModule.ErrParticipationCutoff, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorConflictWithCode, errors.New(mealplanModule.ErrParticipationCutoff.Detail), apiCommon.CodeMealsParticipationCutoffPassed)},
}

func renderMealPlanFailure(w http.ResponseWriter, r *http.Request, err error, internalMessage string) {
	renderer := apiCommon.RulesRenderer(mealPlanErrorRules, apiCommon.ErrorInternalServerRenderer(internalMessage))
	apiCommon.RenderError(w, r, renderer(err))
}

func newMealPlanResource(module mealplanModule.Capability, renderer SimpleListRenderer) *mealplanAPI.Resource {
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
		ParseIDParam: apiCommon.ParseIDParam,
		Account: func(r *http.Request) (int64, error) {
			principal, err := apiCommon.CurrentPrincipal(r.Context())
			return principal.AccountID(), err
		},
		ModuleFailure: func(w http.ResponseWriter, r *http.Request, err error, internalMessage string) {
			renderMealPlanFailure(w, r, err, internalMessage)
		},
		ExportDailyList: func(list mealplanModule.DailyList, format string) (mealplanAPI.ExportFile, error) {
			return renderMealParticipationExport(renderer, list.Export, format)
		},
		ExportWeeklyList: func(week mealplanModule.WeeklyList, format string) (mealplanAPI.ExportFile, error) {
			return renderMealParticipationExport(renderer, week.Export, format)
		},
	})
}

func renderMealParticipationExport(renderer SimpleListRenderer, layout func() (mealplanModule.DailyListExport, error), format string) (mealplanAPI.ExportFile, error) {
	export, err := layout()
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
		ParseIDParam: apiCommon.ParseIDParam,
	})
}

func newFeedbackResource(module feedbackModule.Capability) *feedbackAPI.Resource {
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
		ParseIDParam: apiCommon.ParseIDParam,
	})
}
