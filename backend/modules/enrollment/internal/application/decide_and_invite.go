package application

import (
	"context"
	"fmt"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// decideFunc applies one child's decision inside the caller's unit.
type decideFunc func(context.Context, enrollmentNative.DecideInput) (*enrollmentNative.DecideOutcome, error)

// DecideAndInvite applies one child's decision and creates the guardian
// invitation an approval owes in the same unit of work (#6093): both commit
// or both roll back. Identity & Access joins the unit through its public
// invitation command (ADR 0067, E1); nothing is left pending for the caller.
//
// The command joins the caller's transaction when there is one. Called
// without one, it opens its own unit and replays it once after a transient
// database failure, but only while the decision body has not run: a failure
// after it, including at commit, is returned rather than risk applying the
// approval twice. An ended caller context stops the retry and is returned.
func (d *Decisions) DecideAndInvite(ctx context.Context, input enrollmentNative.DecideInput) (*enrollmentNative.DecideOutcome, error) {
	return d.decideAndInvite(ctx, input, d.Decide)
}

func (d *Decisions) decideAndInvite(ctx context.Context, input enrollmentNative.DecideInput, decide decideFunc) (*enrollmentNative.DecideOutcome, error) {
	attempts := 2
	if d.deps.Runtime.InTransaction != nil && d.deps.Runtime.InTransaction(ctx) {
		attempts = 1
	}
	for attempt := 1; ; attempt++ {
		outcome, decided, err := d.decideAndInviteOnce(ctx, input, decide)
		if err == nil {
			return outcome, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if decided || attempt >= attempts || d.deps.IsTransient == nil || !d.deps.IsTransient(err) {
			return nil, err
		}
		d.logger().WarnContext(ctx, "transient enrollment decision failure, retrying",
			"request_id", input.RequestID,
			"child_id", input.ChildID,
			"error", err.Error(),
		)
	}
}

// decideAndInviteOnce runs one attempt and reports whether the decision body
// completed before the unit failed.
func (d *Decisions) decideAndInviteOnce(ctx context.Context, input enrollmentNative.DecideInput, decide decideFunc) (*enrollmentNative.DecideOutcome, bool, error) {
	var outcome *enrollmentNative.DecideOutcome
	decided := false
	err := d.deps.Runtime.Transactions.RunInTx(ctx, func(txCtx context.Context) error {
		out, err := decide(txCtx, input)
		if err != nil {
			return err
		}
		decided = true
		if err := d.createOwedInvitation(txCtx, out.PendingInvite); err != nil {
			return err
		}
		out.PendingInvite = nil
		outcome = out
		return nil
	})
	if err != nil {
		return nil, decided, err
	}
	return outcome, decided, nil
}

func (d *Decisions) createOwedInvitation(ctx context.Context, invite *enrollmentNative.PendingGuardianInvite) error {
	if invite == nil || d.deps.GuardianInvitations == nil {
		return nil
	}
	if _, err := d.deps.GuardianInvitations.CreateGuardianInvitation(ctx, identityaccess.GuardianInvitationRequest{
		GuardianProfileID: invite.GuardianProfileID, CreatedBy: invite.CreatedBy,
	}); err != nil {
		return fmt.Errorf("decision: create guardian invitation: %w", err)
	}
	return nil
}
