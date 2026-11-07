package owners

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// careContractChangeSettingReasonPolicy is the school's rule whether a
// decision of a parent request needs a reason (#2267).
const careContractChangeSettingReasonPolicy = "operations.parent_request_reason_policy"

// careContractChangeActorRole is the actor role of a decision by the
// Träger-Büro, as the Träger-Portal names it in the access log.
const careContractChangeActorRole = "traeger"

// CareContractChangeSettings resolves a setting of the school whose tenant
// transaction the caller holds.
type CareContractChangeSettings interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

// CareContractChangeRequests binds the Träger-Büro's decision of the
// parents' changes (#5979) to Care Plan's offering-change review: the same
// read the parents portal shows and the same Decide the school's review
// calls, in the tenant transaction of the contract's school the caller
// holds. Care Plan locks the request and checks the child under that lock.
type CareContractChangeRequests struct {
	Changes  careplan.OfferingChangeCapability
	Settings CareContractChangeSettings
}

var _ carecontract.SchoolChangeRequests = CareContractChangeRequests{}

// PendingChanges lists the child's open change with its diff and the last
// decision, and whether an approval needs a reason at this school.
func (b CareContractChangeRequests) PendingChanges(ctx context.Context, studentID int64) ([]carecontract.ChangeRequest, *carecontract.ChangeOutcome, bool, error) {
	view, err := b.Changes.GetForStudent(ctx, studentID)
	if err != nil {
		return nil, nil, false, err
	}
	reasonRequired, err := b.reasonRequired(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	pending := []carecontract.ChangeRequest{}
	if view == nil {
		return pending, nil, reasonRequired, nil
	}
	if view.Request != nil {
		request, mapErr := careContractChangeRequest(*view.Request, view.Diff)
		if mapErr != nil {
			return nil, nil, false, mapErr
		}
		pending = append(pending, request)
	}
	var last *carecontract.ChangeOutcome
	if decision := view.LastDecision; decision != nil {
		last = &carecontract.ChangeOutcome{
			Approved: decision.Status == careplan.OfferingChangeApproved, DecidedAt: decision.DecidedAt,
			EffectiveFrom: decision.EffectiveFrom, Reason: decision.Reason,
		}
	}
	return pending, last, reasonRequired, nil
}

// DecideChange decides the request as the school's review would, with the
// reach of the Träger-Büro: every child of the school. The caller checked
// the right and that the school belongs to the Träger; Care Plan checks
// that the request is the child's.
func (b CareContractChangeRequests) DecideChange(ctx context.Context, decision carecontract.ChangeDecision) error {
	reasonRequired, err := b.reasonRequired(ctx)
	if err != nil {
		return err
	}
	_, err = b.Changes.Decide(withTraegerChangeReview(ctx), careplan.OfferingChangeDecisionInput{
		RequestID: decision.RequestID, StudentID: decision.StudentID, Approve: decision.Approve,
		Reason: decision.Reason, ReviewedBy: decision.AccountID, ActorRole: careContractChangeActorRole,
		EffectiveFrom: decision.EffectiveFrom, ExpectedVersion: decision.ExpectedVersion,
		ReasonRequired: reasonRequired,
	})
	return careContractChangeError(err)
}

func (b CareContractChangeRequests) reasonRequired(ctx context.Context) (bool, error) {
	policy, err := b.Settings.ResolveString(ctx, careContractChangeSettingReasonPolicy)
	if err != nil {
		return false, fmt.Errorf("care contract changes: resolve reason policy: %w", err)
	}
	return parentrequests.ReasonRequired(policy, parentrequests.ReasonSideStaff), nil
}

func careContractChangeRequest(request careplan.OfferingChangeRequest, diff []careplan.OfferingChangeDiffEntry) (carecontract.ChangeRequest, error) {
	effectiveFrom, err := calendar.ParseDate(request.EffectiveFrom)
	if err != nil {
		return carecontract.ChangeRequest{}, fmt.Errorf("care contract changes: effective day of request %d: %w", request.ID, err)
	}
	result := carecontract.ChangeRequest{
		ID: request.ID, Version: careplan.ParentRequestVersion(request.UpdatedAt), SubmittedAt: request.CreatedAt,
		EffectiveFrom: effectiveFrom, EndsAllCare: request.CompleteWithdrawalConfirmed, Lines: []carecontract.ChangeLine{},
	}
	if request.ParentNote != nil {
		result.ParentNote = *request.ParentNote
	}
	for _, entry := range diff {
		result.Lines = append(result.Lines, carecontract.ChangeLine{
			CareOfferingID: entry.OfferingID, ModuleName: entry.Label, State: careContractChangeState(entry),
			BeforeDays: nonNilDays(entry.OldDays), AfterDays: nonNilDays(entry.NewDays),
		})
	}
	return result, nil
}

func careContractChangeState(entry careplan.OfferingChangeDiffEntry) string {
	booked, wanted := entry.OldState == "booked", entry.NewState == "booked"
	switch {
	case !booked && wanted:
		return carecontract.ChangeModuleAdded
	case booked && !wanted:
		return carecontract.ChangeModuleRemoved
	case len(entry.OldDays) == len(entry.NewDays) && fmt.Sprint(entry.OldDays) == fmt.Sprint(entry.NewDays):
		return carecontract.ChangeModuleUnchanged
	default:
		return carecontract.ChangeModuleChanged
	}
}

func nonNilDays(days []string) []string {
	if days == nil {
		return []string{}
	}
	return days
}

// careContractChangeErrors translate Care Plan's failures of a decision
// into the ones the office can act on; the original stays wrapped.
var careContractChangeErrors = []struct {
	from []error
	to   error
}{
	{[]error{careplan.ErrOfferingChangeNotFound, careplan.ErrOfferingChangeForbidden}, carecontract.ErrChangeNotFound},
	{[]error{careplan.ErrOfferingChangeNotPending}, carecontract.ErrChangeDecided},
	{[]error{parentrequests.ErrStale, careplan.ErrParentRequestStale}, carecontract.ErrChangeStale},
	{[]error{parentrequests.ErrPast, careplan.ErrParentRequestPast}, carecontract.ErrChangePast},
	{[]error{parentrequests.ErrReasonRequired, careplan.ErrParentRequestReasonRequired}, carecontract.ErrChangeReasonRequired},
	{[]error{careplan.ErrCompleteWithdrawalConfirmationRequired}, carecontract.ErrChangeEndsCare},
	{[]error{careplan.ErrOfferingChangeCapacityFull}, carecontract.ErrChangeCapacityFull},
	{[]error{careplan.ErrOfferingChangeDateOutOfRange}, carecontract.ErrChangeDateOutOfRange},
	{[]error{careplan.ErrOfferingChangeNoEnrollment, careplan.ErrCareOfferingDayConflict, careplan.ErrCareOfferingsDisabled,
		careplan.ErrOfferingChangeDisabled}, carecontract.ErrChangeNotApplicable},
	{[]error{careplan.ErrOfferingChangeInvalid, careplan.ErrOfferingAdjustmentInvalid}, carecontract.ErrChangeInvalid},
}

func careContractChangeError(err error) error {
	if err == nil {
		return nil
	}
	for _, rule := range careContractChangeErrors {
		for _, from := range rule.from {
			if errors.Is(err, from) {
				return fmt.Errorf("%w: %w", rule.to, err)
			}
		}
	}
	return err
}

// traegerChangeReviewKey marks a decision the Träger-Büro takes through
// CareContractChangeRequests. Only this package sets it.
type traegerChangeReviewKey struct{}

func withTraegerChangeReview(ctx context.Context) context.Context {
	return context.WithValue(ctx, traegerChangeReviewKey{}, true)
}

// traegerChangeReview tells that the Träger-Büro decides: it reaches every
// child of the school, which the caller established through the right
// "Verträge verwalten" and the school's Träger.
func traegerChangeReview(ctx context.Context) bool {
	marked, _ := ctx.Value(traegerChangeReviewKey{}).(bool)
	return marked
}
