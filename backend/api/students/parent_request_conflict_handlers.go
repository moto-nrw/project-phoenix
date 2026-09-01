package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Conflict resolution (#2267, stories 6-10). The list marks which open
// requests of one child contradict each other (conflict_keys); this route
// closes such a group in ONE transaction, so the family can never end up with
// the result of whichever request happened to be decided last.
//
// The three outcomes are mutually exclusive and the body carries exactly one:
// chosen_request_id (that wish wins), staff_value (the staff member's own
// result) or none (nothing changes). Every request not chosen is rejected,
// which is why the reason is always required.

type resolveConflictBody struct {
	Kind string `json:"kind"`
	// RequestIDs and ExpectedVersions are positional pairs, echoed back from
	// the list. Ids travel as strings like every other int64 on this API.
	RequestIDs       []string       `json:"request_ids"`
	ExpectedVersions []string       `json:"expected_versions"`
	ChosenRequestID  string         `json:"chosen_request_id"`
	StaffValue       map[string]any `json:"staff_value"`
	None             bool           `json:"none"`
	// ConflictKey is the group's key from the list. It pins WHICH part of a
	// multi-key request (a weekly plan touching several weekdays) a staff
	// value is written against.
	ConflictKey string `json:"conflict_key"`
	Reason      string `json:"reason"`
}

// resolveRequestConflict decides a whole conflict group at once.
func (rs *Resource) resolveRequestConflict(w http.ResponseWriter, r *http.Request) {
	if rs.ParentRequestConflictService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("parent request conflict service not configured")))
		return
	}
	var body resolveConflictBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	input, err := body.toInput()
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	input.ReviewerID = int64(claims.ID)
	input.ActorRole = strings.Join(claims.Roles, ",")
	if err := rs.ParentRequestConflictService.ResolveConflict(r.Context(), input); err != nil {
		tenant.MarkRollback(r.Context())
		renderError(w, r, resolveConflictErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int{"resolved_count": len(input.RequestIDs)}, "Conflict resolved")
}

// toInput parses the wire body. Only shape errors are decided here — whether
// the command makes sense (one outcome, versions matching, one child) is the
// coordinator's call, so the rules cannot drift between the two.
func (b *resolveConflictBody) toInput() (userService.ResolveConflictInput, error) {
	requestIDs, err := parseConflictIDs(b.RequestIDs)
	if err != nil {
		return userService.ResolveConflictInput{}, err
	}
	var chosen int64
	if strings.TrimSpace(b.ChosenRequestID) != "" {
		chosen, err = strconv.ParseInt(b.ChosenRequestID, 10, 64)
		if err != nil || chosen <= 0 {
			return userService.ResolveConflictInput{}, errors.New("invalid chosen request id")
		}
	}
	return userService.ResolveConflictInput{
		Kind:             userService.ParentRequestKind(b.Kind),
		RequestIDs:       requestIDs,
		ExpectedVersions: b.ExpectedVersions,
		ChosenRequestID:  chosen,
		StaffValue:       b.StaffValue,
		None:             b.None,
		ConflictKey:      strings.TrimSpace(b.ConflictKey),
		Reason:           strings.TrimSpace(b.Reason),
	}, nil
}

func parseConflictIDs(raw []string) ([]int64, error) {
	ids := make([]int64, 0, len(raw))
	for _, value := range raw {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.New("invalid request id")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// resolveConflictErrorRenderer answers for five domains at once, so it carries
// the union of their sentinels next to the shared parent-request codes.
var resolveConflictErrorRenderer = common.RulesRenderer(parentRequestRules(
	common.ErrorRule{Target: userService.ErrInvalidConflictResolution, Render: common.ErrorInvalidRequest},
	common.ErrorRule{Target: userService.ErrConflictKindUnsupported, Render: func(error) render.Renderer {
		return common.ErrorInvalidRequestMessageWithCode(
			"Für diese Art von Anfrage kann kein gemeinsames Ergebnis festgelegt werden.",
			codeConflictKindUnsupported,
		)
	}},
	common.ErrorRule{Target: userService.ErrStaffValueUnsupported, Render: func(error) render.Renderer {
		return common.ErrorInvalidRequestMessageWithCode(
			"Für diese Anfragen können Sie keinen eigenen Wert eintragen.",
			codeStaffValueUnsupported,
		)
	}},
	common.ErrorRule{Match: isConflictStaffValueInvalid, Render: func(error) render.Renderer {
		return common.ErrorInvalidRequestMessageWithCode(
			"Der eingetragene Wert passt nicht zu diesen Anfragen. Bitte prüfen Sie ihn.",
			codeStaffValueInvalid,
		)
	}},
	common.ErrorRule{Target: userService.ErrParentRequestNotFound, Render: common.ErrorNotFound},
	common.ErrorRule{Match: isParentRequestMissing, Render: common.ErrorNotFound},
	common.ErrorRule{Match: isParentRequestNotPending, Render: conflictWithCode("change_request_not_pending")},
	common.ErrorRule{Match: isParentRequestForbidden, Render: common.ErrorForbidden},
	common.ErrorRule{Target: userService.ErrParentRequestForbidden, Render: common.ErrorForbidden},
), common.ErrorInternalServer)

// isConflictStaffValueInvalid matches the five domains' "the value you typed
// is not usable" sentinels. They are client errors, never 500s.
func isConflictStaffValueInvalid(err error) bool {
	return errors.Is(err, userService.ErrReviewInvalidValue) ||
		errors.Is(err, userService.ErrReviewInvalidTarget) ||
		errors.Is(err, absenceService.ErrAbsenceRequestInvalidStatus) ||
		errors.Is(err, scheduleService.ErrInvalidCareRequestPayload) ||
		errors.Is(err, enrollmentService.ErrOfferingChangeInvalid)
}
