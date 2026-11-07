package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ActionSetEnd is the trail action of the end of an extraordinary
// termination.
const ActionSetEnd = ports.ActionSetEnd

// SetNoticeEnd sets the last day of a contract the parents terminated
// extraordinarily, once the Träger-Büro checked their reason (#4682,
// § 312k Abs. 2 BGB). The end lies on or after the day the declaration
// arrived. It takes the contract's row lock first, as every declaration
// does, so a second end waits and then finds none open.
func (n *Notices) SetNoticeEnd(ctx context.Context, reviewer carecontract.Reviewer, contractID int64, endsOn calendar.Date) (carecontract.NoticeOptions, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	if err := n.requireOffice(); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	var options carecontract.NoticeOptions
	err := n.office.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		record, err := n.store.Contract(txCtx, reviewer.OrganizationID, contractID, true)
		if err != nil {
			return err
		}
		state, err := n.notices.NoticeState(txCtx, reviewer.OrganizationID, contractID)
		if err != nil {
			return err
		}
		notice, err := openEnd(record, state.Notice, endsOn)
		if err != nil {
			return err
		}
		if err := n.notices.SetNoticeEnd(txCtx, reviewer.OrganizationID, contractID, endsOn); err != nil {
			return err
		}
		if err := n.trail(txCtx, reviewer.OrganizationID, reviewer.AccountID, record, ActionSetEnd,
			map[string]any{"ends_on": endsOn.String(), "termination_kind": notice.TerminationKind}); err != nil {
			return err
		}
		options, err = n.options(txCtx, reviewer.OrganizationID, record, n.today(), true)
		return err
	})
	return options, err
}

// openEnd is the extraordinary termination whose end is still open, when
// endsOn may end it: not before the day the declaration arrived.
func openEnd(record ContractRecord, notice *carecontract.Notice, endsOn calendar.Date) (*carecontract.Notice, error) {
	if record.Summary.Status != carecontract.StatusTerminated || notice == nil ||
		notice.TerminationKind != carecontract.TerminationExtraordinary || notice.EndsOn != nil {
		return nil, carecontract.ErrNoticeEndNotOpen
	}
	if endsOn.IsZero() || endsOn.Before(notice.ReceivedOn) {
		return nil, &carecontract.ValidationError{Fields: map[string]string{"ends_on": "out_of_range"}}
	}
	return notice, nil
}
