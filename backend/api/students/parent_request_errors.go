package students

import (
	"context"
	"errors"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	reviewidentity "github.com/moto-nrw/project-phoenix/modules/identityaccess/requestreview"
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
	// The Care Plan excused-absence workflow (#3093) answers with its own
	// lifecycle sentinels; they render the same wire codes as the shared ones.
	{Target: excusedrequests.ErrParentRequestStale, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, codeChangeRequestStale)
	}},
	{Target: excusedrequests.ErrParentRequestReasonRequired, Render: func(error) render.Renderer {
		return common.ErrorInvalidRequestMessageWithCode(
			"Bitte tragen Sie eine Begründung ein.",
			codeReasonRequired,
		)
	}},
	{Target: excusedrequests.ErrParentRequestPast, Render: func(error) render.Renderer {
		return common.ErrorConflictMessageWithCode(
			"Diese Anfrage betrifft nur vergangene Tage. Sie können sie nur ablehnen oder als erledigt markieren.",
			codeRequestPast,
		)
	}},
	{Target: excusedrequests.ErrParentRequestNotPast, Render: func(error) render.Renderer {
		return common.ErrorConflictMessageWithCode(
			"Diese Anfrage betrifft noch kommende Tage. Bitte entscheiden Sie sie.",
			codeRequestNotPast,
		)
	}},
	{Target: excusedrequests.ErrParentRequestNotDecided, Render: func(error) render.Renderer {
		return common.ErrorConflictMessageWithCode(
			"Diese Anfrage ist noch nicht entschieden. Es gibt nichts zu korrigieren.",
			codeRequestNotDecided,
		)
	}},
	{Target: excusedrequests.ErrParentRequestCorrectionUnsupported, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, codeCorrectionUnsupp)
	}},
	{Target: authorize.ErrAbsenceReadRequired, Render: absenceReadRequiredResponse},
	{Target: reviewidentity.ErrAbsenceReadRequired, Render: absenceReadRequiredResponse},
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

func absenceReadRequiredResponse(error) render.Renderer {
	return common.ErrorForbiddenMessageWithCode(
		"Für Elternanfragen zu Abwesenheiten brauchen Sie zusätzlich das Recht „Kinder sehen“.",
		codeAbsenceReadRequird,
	)
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
	return errors.Is(err, excusedrequests.ErrExcusedRequestNotFound) ||
		errors.Is(err, scheduleModels.ErrCareRequestNotFound) ||
		errors.Is(err, enrollmentModels.ErrOfferingChangeNotFound) ||
		errors.Is(err, userService.ErrReviewNotFound)
}

func isParentRequestNotPending(err error) bool {
	return errors.Is(err, excusedrequests.ErrExcusedRequestNotPending) ||
		errors.Is(err, scheduleModels.ErrCareRequestNotPending) ||
		errors.Is(err, enrollmentModels.ErrOfferingChangeNotPending) ||
		errors.Is(err, userService.ErrReviewNotPending)
}

func isParentRequestForbidden(err error) bool {
	return errors.Is(err, excusedrequests.ErrExcusedRequestForbidden) ||
		errors.Is(err, scheduleService.ErrCareRequestForbidden) ||
		errors.Is(err, enrollmentService.ErrOfferingChangeForbidden) ||
		errors.Is(err, userService.ErrReviewForbidden)
}

// isParentRequestNotDecided matches the union of the domains' "there is no
// decision here to correct" sentinels.
func isParentRequestNotDecided(err error) bool {
	return errors.Is(err, excusedrequests.ErrExcusedRequestNotDecided) || errors.Is(err, excusedrequests.ErrParentRequestNotDecided) ||
		errors.Is(err, userService.ErrParentRequestNotDecided)
}
