package application

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// errStudentConsentAuditRequired marks a consent write whose history could
// not be recorded; the approval must roll back instead of committing it.
var errStudentConsentAuditRequired = errors.New("student consent audit is required")

// consentStamp is one consent flag and the student timestamps it sets.
type consentStamp struct {
	key   string
	apply func(student *Student, at *time.Time, reviewedBy int64)
}

// consentStamps copies every present consent flag onto the student row so
// staff looking at a single child see the consent state without joining
// back to enrollment.requests. The public form submits only configured legal
// blocks, so absent flags simply leave the matching timestamp null. Each
// stamp records the approval moment, not the parent submission moment — the
// parent submission timestamp lives on enrollment.requests.
var consentStamps = []consentStamp{
	{key: enrollmentNative.ConsentKeyPhoto, apply: func(student *Student, at *time.Time, reviewedBy int64) {
		student.PhotoConsentGivenAt = at
		if at == nil {
			student.PhotoConsentGivenBy = nil
		} else if reviewedBy > 0 {
			rb := reviewedBy
			student.PhotoConsentGivenBy = &rb
		}
	}},
	{key: enrollmentNative.ConsentKeyAGB, apply: func(student *Student, at *time.Time, _ int64) {
		student.AGBAcceptedAt = at
	}},
	{key: enrollmentNative.ConsentKeyDataProcessing, apply: func(student *Student, at *time.Time, _ int64) {
		student.DataProcessingAcceptedAt = at
	}},
	{key: enrollmentNative.ConsentKeyEmailContact, apply: func(student *Student, at *time.Time, _ int64) {
		student.EmailContactAcceptedAt = at
	}},
}

// applyConsentFlags takes the core base-form consent flags onto the student.
// They don't appear in the Stammdaten-target picker, but their values still
// need to land in the right downstream rows on approval.
//
// Withdrawal: a replacing submission answers the whole consent question, so
// a block the guardian left unchecked must CLEAR the matching timestamp
// instead of leaving last year's consent standing. Clearing keys on PRESENCE,
// not on falsiness: the form submits every configured legal block (an
// unchecked optional box arrives as an explicit false) and the submission
// drops everything else, so an absent key means "this form never asked" —
// not "withdrawn". Wiping those would let a renewal phase that configures no
// photo block erase a photo consent the original enrollment recorded (#1663).
func (r *targetedFieldSync) applyConsentFlags() {
	flags := r.request.ConsentFlags
	if flags.IsZero() || r.options.Scope == weeklyPlanOnly {
		return
	}
	now := time.Now()
	for _, stamp := range consentStamps {
		if given, ok := formValue(flags, stamp.key).(bool); ok && given {
			stamp.apply(r.student, &now, r.reviewedBy)
			r.studentDirty = true
		}
	}
	if !r.options.Replace && !r.options.ReplaceConsent {
		return
	}
	for _, stamp := range consentStamps {
		if given, ok := formValue(flags, stamp.key).(bool); ok && !given {
			stamp.apply(r.student, nil, r.reviewedBy)
			r.studentDirty = true
		}
	}
}

// persist writes the dirty student, records the consent transitions and
// assembles the dispatch's errors.
//
// The companion refusals are kept as a separately WRAPPED error, not joined
// into the field errors: the enrollment departure workflow reconciles the "läuft mit"
// edges for every caller, and these two sentinels are expected,
// user-actionable refusals (fix the other child's Heimweg first / retry after
// the concurrent edit). A caller that tolerates field errors still has to
// return them, or a legitimate enrollment change would end as an opaque 500
// at the handler (#1694).
func (r *targetedFieldSync) persist(before *Student) (bool, error) {
	departurePlanSynced, studentUpdated, companionRefusal := r.writeStudent(before)
	if studentUpdated && r.d.deps.StudentConsents != nil {
		var actorAccountID *int64
		if r.reviewedBy > 0 {
			actor := r.reviewedBy
			actorAccountID = &actor
		}
		if err := r.d.deps.StudentConsents.RecordEnrollmentConsentTransitions(r.ctx, before, r.student, actorAccountID, time.Now()); err != nil {
			return departurePlanSynced, fmt.Errorf("%w: %w", errStudentConsentAuditRequired, err)
		}
	}
	if companionRefusal != nil {
		if len(r.errs) > 0 {
			return departurePlanSynced, fmt.Errorf("%w; %w", fieldErrors(r.errs), companionRefusal)
		}
		return departurePlanSynced, companionRefusal
	}
	if len(r.errs) > 0 {
		return departurePlanSynced, fieldErrors(r.errs)
	}
	return departurePlanSynced, nil
}

// fieldErrors joins the dispatch's field errors into one "; "-separated
// message and keeps every error in the chain, so a caller can still tell
// invalid request data (ErrDecisionInvalidData) from an infrastructure
// failure.
type fieldErrors []error

func (e fieldErrors) Error() string {
	msgs := make([]string, len(e))
	for i, err := range e {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "; ")
}

func (e fieldErrors) Unwrap() []error { return e }

// writeStudent writes a dirty student. Carrying a departure plan is a
// NECESSARY, not a sufficient, condition for a companion change: writing the
// same modes back trims no edge, and announcing student_companions_changed
// for such a write makes every open companion editor in the school discard or
// block its draft for nothing. Only the write path knows the difference, so
// it is read from the write's companion change recorder.
func (r *targetedFieldSync) writeStudent(before *Student) (planSynced, updated bool, companionRefusal error) {
	if !r.studentDirty {
		return false, false, nil
	}
	updateCtx, companionChanges := contextWithCompanionChangeRecorder(r.ctx)
	var updateErr error
	switch {
	case r.d.deps.StudentEnrollment == nil:
		updateErr = errors.New("decision: student enrollment capability is required")
	case departurePlanChanged(before, r.student):
		updateErr = r.d.applyEnrollmentDeparture(updateCtx, before, r.student)
	default:
		updateErr = r.d.deps.StudentEnrollment.ApplyEnrollmentProfile(updateCtx, r.student.ID, enrollmentProfilePatch(before, r.student))
	}
	if updateErr == nil {
		return companionChanges.changed, true, nil
	}
	if r.d.isCompanionRefusal(updateErr) {
		return false, false, fmt.Errorf("update student: %w", updateErr)
	}
	r.errs = append(r.errs, fmt.Errorf("update student: %w", updateErr))
	return false, false, nil
}

// departurePlanChanged reports whether the dispatch changed the departure
// plan or its companion note.
func departurePlanChanged(before, after *Student) bool {
	return !reflect.DeepEqual(before.AllowedDepartureModes, after.AllowedDepartureModes) ||
		!reflect.DeepEqual(before.DepartureDays, after.DepartureDays) ||
		!reflect.DeepEqual(before.BusDays, after.BusDays) ||
		!reflect.DeepEqual(before.PickupDays, after.PickupDays) ||
		enrollmentValueChanged(before.DepartureCompanionNote, after.DepartureCompanionNote)
}

// isCompanionRefusal reports the People Directory's two companion refusals.
func (d *Decisions) isCompanionRefusal(err error) bool {
	people := d.deps.People
	return (people.ErrCompanionWouldLoseDeparture != nil && errors.Is(err, people.ErrCompanionWouldLoseDeparture)) ||
		(people.ErrCompanionLockBusy != nil && errors.Is(err, people.ErrCompanionLockBusy))
}

// companionChangeRecorder answers "did this write actually change a child's
// 'läuft mit' links?" for the departure sync, which trims the links itself.
// Scope is one transaction — the recorder lives in the context of the write
// and is never shared across requests or goroutines.
type companionChangeRecorder struct {
	changed bool
}

type companionChangeRecorderKey struct{}

func contextWithCompanionChangeRecorder(ctx context.Context) (context.Context, *companionChangeRecorder) {
	recorder := &companionChangeRecorder{}
	return context.WithValue(ctx, companionChangeRecorderKey{}, recorder), recorder
}

// recordCompanionChange marks the open recording scope as having changed the
// links; a no-op outside a scope.
func recordCompanionChange(ctx context.Context) {
	if recorder, ok := ctx.Value(companionChangeRecorderKey{}).(*companionChangeRecorder); ok && recorder != nil {
		recorder.changed = true
	}
}
