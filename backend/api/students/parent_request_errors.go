package students

import (
	"context"
	"errors"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// Wire error codes shared by every parent-request decide, correct, resolve
// and mark-done route. They are a contract with the frontend — the client
// branches on the code, never on the German message.
const (
	codeChangeRequestStale = "change_request_stale"
	codeAbsenceReadRequird = "absence_read_required"
	codeReasonRequired     = "reason_required"
	codeRequestPast        = "request_past"
	codeRequestNotPast     = "request_not_past"
	codeRequestNotDecided  = "request_not_decided"
	codeCorrectionUnsupp   = "correction_unsupported"
	// Conflict-resolution codes (#2267, stories 6-10).
	codeConflictKindUnsupported = "conflict_kind_unsupported"
	codeStaffValueUnsupported   = "staff_value_unsupported"
	codeStaffValueInvalid       = "staff_value_invalid"
)

// parentRequestSharedRules are the rules every parent-request route shares.
// Per-route tables prepend their own domain sentinels and then append these,
// so one sentinel can never render two different codes on two routes.
var parentRequestSharedRules = []common.ErrorRule{
	{Target: userService.ErrParentRequestStale, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, codeChangeRequestStale)
	}},
	{Target: authorize.ErrAbsenceReadRequired, Render: func(error) render.Renderer {
		return common.ErrorForbiddenMessageWithCode(
			"Für Elternanfragen zu Abwesenheiten brauchen Sie zusätzlich das Recht „Kinder sehen“.",
			codeAbsenceReadRequird,
		)
	}},
	{Target: userService.ErrParentRequestReasonRequired, Render: func(error) render.Renderer {
		return common.ErrorInvalidRequestMessageWithCode(
			"Bitte tragen Sie eine Begründung ein.",
			codeReasonRequired,
		)
	}},
	{Target: userService.ErrParentRequestPast, Render: func(error) render.Renderer {
		return common.ErrorConflictMessageWithCode(
			"Diese Anfrage betrifft nur vergangene Tage. Sie können sie nur ablehnen oder als erledigt markieren.",
			codeRequestPast,
		)
	}},
	{Target: userService.ErrParentRequestNotPast, Render: func(error) render.Renderer {
		return common.ErrorConflictMessageWithCode(
			"Diese Anfrage betrifft noch kommende Tage. Bitte entscheiden Sie sie.",
			codeRequestNotPast,
		)
	}},
	{Target: userService.ErrParentRequestNotDecided, Render: func(error) render.Renderer {
		return common.ErrorConflictMessageWithCode(
			"Diese Anfrage ist noch nicht entschieden. Es gibt nichts zu korrigieren.",
			codeRequestNotDecided,
		)
	}},
	{Target: userService.ErrParentRequestCorrectionUnsupported, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, codeCorrectionUnsupp)
	}},
}

// parentRequestRules builds one route's table: its own sentinels first (more
// specific wins), then the shared ones.
func parentRequestRules(own ...common.ErrorRule) []common.ErrorRule {
	rules := make([]common.ErrorRule, 0, len(own)+len(parentRequestSharedRules))
	rules = append(rules, own...)
	return append(rules, parentRequestSharedRules...)
}

// parentRequestQueueErrorRenderer is what the read surfaces (aggregated list,
// pending-count badge) render. It carries no domain sentinels: what a queue
// can fail on beyond a DB error is the review policy refusing the caller.
var parentRequestQueueErrorRenderer = common.RulesRenderer(
	parentRequestRules(), common.ErrorInternalServer,
)

// ParentRequestReviewAccess is the port the aggregated list uses to explain an
// empty queue. Implemented by usercontext.ParentRequestReviewPolicy.
type ParentRequestReviewAccess interface {
	AccessLevel(ctx context.Context, permissions []string) (string, error)
}

// The cross-kind predicates the lifecycle routes classify by. One route serves
// four domains, so it matches the union of their sentinels rather than
// repeating four tables.

func isParentRequestMissing(err error) bool {
	return errors.Is(err, activeModels.ErrExcusedRequestNotFound) ||
		errors.Is(err, scheduleModels.ErrCareRequestNotFound) ||
		errors.Is(err, enrollmentModels.ErrOfferingChangeNotFound) ||
		errors.Is(err, userService.ErrReviewNotFound)
}

func isParentRequestNotPending(err error) bool {
	return errors.Is(err, activeModels.ErrExcusedRequestNotPending) ||
		errors.Is(err, scheduleModels.ErrCareRequestNotPending) ||
		errors.Is(err, enrollmentModels.ErrOfferingChangeNotPending) ||
		errors.Is(err, userService.ErrReviewNotPending)
}

func isParentRequestForbidden(err error) bool {
	return errors.Is(err, absenceService.ErrExcusedRequestForbidden) ||
		errors.Is(err, scheduleService.ErrCareRequestForbidden) ||
		errors.Is(err, enrollmentService.ErrOfferingChangeForbidden) ||
		errors.Is(err, userService.ErrReviewForbidden)
}

// isParentRequestNotDecided matches the union of the domains' "there is no
// decision here to correct" sentinels.
func isParentRequestNotDecided(err error) bool {
	return errors.Is(err, activeModels.ErrExcusedRequestNotDecided) ||
		errors.Is(err, userService.ErrParentRequestNotDecided)
}
