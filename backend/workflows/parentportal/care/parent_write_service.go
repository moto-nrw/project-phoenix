package care

import (
	"errors"
)

// Sentinel errors the HTTP layer maps to stable status codes. They are
// part of the package contract — handlers switch on them via errors.Is.
var (
	// ErrSickNoteDisabled means operations.parent_sick_note_enabled is
	// off for the child's tenant.
	ErrSickNoteDisabled = errors.New("parent: sick notes disabled for this school")
	// ErrMealPlanDisabled means operations.meal_plan_enabled is off for the
	// child's tenant, so the parents portal must hide the meal plan section.
	ErrMealPlanDisabled = errors.New("parent: meal plan disabled for this school")
	// ErrMealPlanWeekOutOfRange means the requested week is outside the window
	// parents may view (the current and next work week). Staff may plan
	// arbitrary future weeks on the staff page, but those are drafts; the
	// parents portal only ever exposes this week and next, so a request for any
	// other week is refused rather than leaking an unpublished menu.
	ErrMealPlanWeekOutOfRange      = errors.New("parent: meal plan week is outside the viewable range")
	ErrMealRegistrationDisabled    = errors.New("parent: meal registration disabled for this school")
	ErrMealParticipationOutOfRange = errors.New("parent: meal participation date is outside the changeable range")
	ErrMealParticipationCutoff     = errors.New("parent: meal participation cutoff has passed")
	ErrInvalidMealParticipation    = errors.New("parent: invalid meal participation")
	// ErrNoDates means the sick-note request carried no dates.
	ErrNoDates = errors.New("parent: at least one date is required")
	// ErrInvalidStatus means the absence status was neither sick nor excused.
	ErrInvalidStatus = errors.New("parent: status must be sick or excused")
	// ErrPickupChangeDisabled means operations.parent_pickup_change_enabled is
	// off for the child's tenant.
	ErrPickupChangeDisabled = errors.New("parent: pickup-time change disabled for this school")
	// ErrNoCareException means the request carried neither a pickup nor an
	// arrival time.
	ErrNoCareException = errors.New("parent: at least one of pickup or arrival time is required")
	// ErrCareExceptionReasonRequired means the parent API request omitted the
	// explanation staff need to understand a changed pickup time.
	ErrCareExceptionReasonRequired = errors.New("parent: care exception reason is required")
	ErrCareExceptionReasonTooLong  = errors.New("parent: care exception reason exceeds 255 characters")
	ErrCareExceptionAlreadyLeft    = errors.New("parent: child has already left care today")
	// ErrPastCareDate means the requested date is in the past.
	ErrPastCareDate = errors.New("parent: care exception date must not be in the past")
	// ErrCareDateTooFar means the requested date is beyond the window parents may
	// set (two calendar months ahead, matching the parent-portal list range).
	ErrCareDateTooFar = errors.New("parent: care exception date is too far in the future")
	// ErrCareExceptionConflict means a staff-authored exception already exists
	// for the date, so the parent change is refused rather than overwriting it.
	ErrCareExceptionConflict = errors.New("parent: the school already set a special time for this day")
	// ErrCareExceptionRaced means two submits for the same child+date collided
	// on the unique index (e.g. a double-click); the change was not saved and
	// the caller should reload and retry.
	ErrCareExceptionRaced = errors.New("parent: this day was just changed, please reload and try again")
	// ErrExcusedRequestNotFound is the legacy-named error for an absence request
	// the caller did not submit for this child.
	ErrExcusedRequestNotFound = errors.New("parent: excused absence request not found")
	// ErrExcusedRequestNotPending means the legacy-named absence request was
	// already decided or withdrawn, so it can no longer be withdrawn.
	ErrExcusedRequestNotPending = errors.New("parent: excused absence request is not pending")
	// ErrExcusedRequestOverlap means a different pending absence request already
	// covers one of the submitted dates. An identical resubmit is idempotent.
	ErrExcusedRequestOverlap = errors.New("parent: excused absence request overlaps an existing pending request")
)
