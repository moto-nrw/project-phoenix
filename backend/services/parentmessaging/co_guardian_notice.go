package parentmessaging

import (
	"context"
)

// Co-guardian neutral notice (#2267, story 47).
//
// A decision on one guardian's request changes the child's care for the whole
// family, but only the guardian who filed it hears about it. The others find
// out when the child is not picked up. So every other guardian with portal
// access gets a NEUTRAL notice: something about this child's care changed, on
// these days.
//
// Neutral means neutral. No reason, no author, no reference to the underlying
// request — those belong to the guardian who wrote it, and a co-guardian who
// was not made an explicit recipient never sees them. A guardian who WAS made
// a recipient sees the full picture through the sharing path; this notice is
// what everyone else gets, and it is the same for a protected child as for an
// unprotected one, because "the care changed" is not private to one parent.
//
// Push stays with the submitting guardian: a neutral line is worth a badge in
// the app, not a phone notification.
//
// The work is split in two on purpose. Resolving WHO gets the notice is a
// tenant-scoped read and must happen inside the caller's transaction; posting
// the pills must happen after that transaction commits. One function doing
// both would have to pick one context and get the other wrong.

// OtherPortalGuardianAccountIDs returns the accounts of every portal guardian
// of this child EXCEPT the submitter. Call it on the caller's context, inside
// the tenant transaction — it is a tenant-scoped read.
func (e *Emitter) OtherPortalGuardianAccountIDs(
	ctx context.Context,
	studentID, submitterAccountID int64,
) ([]int64, error) {
	if e == nil || e.threadRepo == nil || studentID <= 0 {
		return nil, nil
	}
	guardians, err := e.threadRepo.ListGuardiansForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	accountIDs := make([]int64, 0, len(guardians))
	for _, guardian := range guardians {
		if guardian == nil || guardian.AccountID <= 0 || guardian.AccountID == submitterAccountID {
			continue
		}
		accountIDs = append(accountIDs, guardian.AccountID)
	}
	return accountIDs, nil
}

// EmitChildEventToGuardians posts ev into each named guardian's thread. Call
// it after the originating transaction committed, like every other pill.
//
// The event is stripped here rather than at the call sites, so no caller can
// leak a reason or an author into it by forgetting to clear a field.
func (e *Emitter) EmitChildEventToGuardians(
	tenantID, studentID int64,
	accountIDs []int64,
	ev ChildEvent,
) {
	if e == nil || tenantID <= 0 || studentID <= 0 {
		return
	}
	neutral := NeutralChildEvent(ev)
	for _, accountID := range accountIDs {
		if accountID > 0 {
			e.EmitChildEvent(tenantID, studentID, accountID, neutral)
		}
	}
}

// NeutralChildEvent strips everything that belongs to the submitting
// guardian: the staff reason, the reference a client could load the original
// request from, and the structured payload. It also drops the request STATUS —
// a co-guardian filed no request, so "erledigt" would describe something they
// never did; keeping RefTable and RefID empty is additionally what stops this
// from counting as a terminal request event, which by design never creates a
// thread and would leave a co-guardian with no notice at all.
//
// ActorAccountID is deliberately KEPT. parent_messages.sender_account_id is
// NOT NULL with a foreign key, so a pill without one cannot be written; and it
// is not an author leak, because every pill is stored with SenderKind=system
// and the parents API projects sender_kind and sender_name ("System"), never
// the account. What a co-guardian must not learn — WHY and WHAT was asked — is
// what this function removes.
func NeutralChildEvent(ev ChildEvent) ChildEvent {
	return ChildEvent{
		EventType:      ev.EventType,
		ActorKind:      ev.ActorKind,
		ActorAccountID: ev.ActorAccountID,
		Body:           ev.Body,
		RequestType:    ev.RequestType,
	}
}

// DecisionAudience splits a child's other portal guardians into the two groups
// a decision produces: those the parent explicitly shared the request with,
// who get the SAME pill the submitter gets, and everyone else, who gets the
// neutral line.
//
// Resolve it inside the caller's transaction and emit after commit — the
// returned slices are plain account ids precisely so they can cross that
// boundary safely.
type DecisionAudience struct {
	// Full are the explicit recipients: they already see the request, so
	// withholding the reason from them would only make the pill useless.
	Full []int64
	// Neutral is everyone else with portal access.
	Neutral []int64
}

// ResolveDecisionAudience answers who hears what about one decision.
// sharedAccountIDs are the explicit recipients the sharing domain reported;
// nil (an unwired or failing resolver) puts everybody in Neutral, which is the
// safe direction — a guardian seeing less than they were entitled to is a
// nuisance, seeing more is a leak.
//
// Familienschutz needs no branch: while it is on the sharing domain reports no
// explicit recipients, so a protected child's co-guardians are all Neutral.
func (e *Emitter) ResolveDecisionAudience(
	ctx context.Context,
	studentID, submitterAccountID int64,
	sharedAccountIDs []int64,
) (DecisionAudience, error) {
	others, err := e.OtherPortalGuardianAccountIDs(ctx, studentID, submitterAccountID)
	if err != nil {
		return DecisionAudience{}, err
	}
	shared := make(map[int64]bool, len(sharedAccountIDs))
	for _, accountID := range sharedAccountIDs {
		shared[accountID] = true
	}
	var audience DecisionAudience
	for _, accountID := range others {
		if shared[accountID] {
			audience.Full = append(audience.Full, accountID)
			continue
		}
		audience.Neutral = append(audience.Neutral, accountID)
	}
	return audience, nil
}

// EmitDecisionAudience posts the full pill to the explicit recipients and the
// neutral line to everyone else. Call it after the decision committed.
func (e *Emitter) EmitDecisionAudience(
	tenantID, studentID int64,
	audience DecisionAudience,
	full, neutral ChildEvent,
) {
	if e == nil {
		return
	}
	for _, accountID := range audience.Full {
		e.EmitChildEvent(tenantID, studentID, accountID, full)
	}
	e.EmitChildEventToGuardians(tenantID, studentID, audience.Neutral, neutral)
}

// ShareVisibilityResolver names the guardians the submitting parent explicitly
// shared ONE request with. It lives here, next to the audience split that
// consumes it, so the four request domains reference one interface instead of
// each declaring their own copy — and so none of them has to import the
// parents package for a question it only needs one answer to.
//
// Implemented by the parents domain's sharing service and injected by setter,
// which keeps every existing construction site working: an unwired service
// simply reports no explicit recipients, and everybody gets the neutral line.
type ShareVisibilityResolver interface {
	SharedRecipientAccountIDs(ctx context.Context, studentID int64, requestType string, requestID int64) ([]int64, error)
}
