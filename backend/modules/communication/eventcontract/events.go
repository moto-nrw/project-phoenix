package eventcontract

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type ChildEvent struct {
	// EventType discriminates the pill for localized rendering:
	// "request_created" | "request_status" | "sick_note" | "care_exception" |
	// "care_exception_correction".
	EventType string
	// ActorKind is the side that TRIGGERED the event — ActorGuardian for parent
	// actions (submit, withdraw, self-service) or ActorStaff for staff
	// decisions. It stamps EventActorKind (so the other side's unread badge
	// fires even for dual-role accounts) and the thread's last-sender side.
	ActorKind string
	// ActorAccountID is the acting account (guardian or staff).
	ActorAccountID int64
	// Body is the German pill text (the staff portal renders it directly; the
	// parents portal localizes from the structured fields below).
	Body string
	// RequestType / RequestStatus / DecisionReason carry the structured request
	// outcome for localized clients. RequestStatus uses the parent_messages
	// vocabulary (offen/erledigt/abgelehnt/zurueckgezogen); callers map their
	// domain statuses at the emit boundary. Empty for non-request events.
	RequestType    string
	RequestStatus  string
	DecisionReason string
	// RefTable / RefID point at the underlying record (e.g. the change-request
	// row) so a future client can deep-link. Optional.
	RefTable string
	RefID    *int64
	// Payload carries the structured detail a localized client needs to render
	// the pill itself instead of the German Body — today the confirmed
	// effective date of a decided offering-change request (#2484). Optional.
	Payload *ChildEventPayload
}

type PortalGuardian struct {
	AccountID    int64
	PortalLocale string
}

type Events interface {
	// EmitChildEvent appends one pill to the (student, guardian) thread in a
	// triggering tenant transaction; only the parent-message wake waits for commit.
	EmitChildEvent(ctx context.Context, tenantID, studentID, guardianAccountID int64, ev ChildEvent) error
	// WakeChildGuardians wakes every portal guardian of the child with a
	// message-independent care-state invalidation.
	WakeChildGuardians(tenantID, studentID int64)
	// PortalGuardians lists the child's portal guardians on the caller's
	// context, so the caller's tenant transaction and RLS scope apply.
	PortalGuardians(ctx context.Context, studentID int64) ([]PortalGuardian, error)
	// InTransaction reports whether ctx carries an active tenant transaction.
	InTransaction(ctx context.Context) bool
	// MessagingEnabledForTenant applies the read-path fail-OPEN gate.
	MessagingEnabledForTenant(ctx context.Context, tenantID int64) bool
}

type DecisionNotifier interface {
	NotifyRequestDecision(ctx context.Context, tenantID, studentID, guardianAccountID int64, locale, refTable string, refID int64, requestType, requestStatus string) error
}

type DecisionAudience struct {
	// Full are the explicit recipients: they already see the request, so
	// withholding the reason from them would only make the pill useless.
	Full []int64
	// Neutral is everyone else with portal access.
	Neutral []int64
}

type ShareVisibilityResolver interface {
	SharedRecipientAccountIDs(ctx context.Context, studentID int64, requestType string, requestID int64) ([]int64, error)
}

type SettingsResolver interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

type TenantSettingsResolver interface {
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
}

const (
	// EventRequestCreated marks a request submission pill.
	EventRequestCreated = "request_created"
	// EventRequestStatus marks a request decision, withdrawal, or close pill.
	EventRequestStatus = "request_status"

	// ActorGuardian attributes a pill to the parent side.
	ActorGuardian = "guardian"
	// ActorStaff attributes a pill to the OGS side.
	ActorStaff = "staff"

	// RequestStatusOpen through RequestStatusWithdrawn are the request
	// outcome vocabulary of the pills.
	RequestStatusOpen      = "offen"
	RequestStatusDone      = "erledigt"
	RequestStatusRejected  = "abgelehnt"
	RequestStatusWithdrawn = "zurueckgezogen"
)

// ChildEventPayload is an opaque JSON detail document. Producers own its
// schema; the timeline preserves it without importing their request types.
// Keeping a marshal failure preserves the existing store-error path.
type ChildEventPayload struct {
	document json.RawMessage
	err      error
}

func NewChildEventPayload(document json.RawMessage, err error) *ChildEventPayload {
	if err == nil && (len(document) == 0 || string(document) == "null") {
		return nil
	}
	return &ChildEventPayload{document: document, err: err}
}
func (p *ChildEventPayload) MarshalJSON() ([]byte, error) {
	if p == nil {
		return []byte("null"), nil
	}
	return p.document, p.err
}

// Emitter is Communication's timeline and decision-delivery capability.
type Emitter interface {
	EmitChildEvent(context.Context, int64, int64, int64, ChildEvent) error
	EnqueueRequestDecision(context.Context, int64, int64, int64, ChildEvent) error
	BroadcastChildUpdateToGuardians(int64, int64)
	MessagingEnabledForTenant(context.Context, int64) bool
	GuardianHasChildAccess(context.Context, int64, int64) (bool, error)
	EmitParentChildEvent(context.Context, int64, int64, int64, communication.ParentChildEvent) error
	OtherPortalGuardianAccountIDs(context.Context, int64, int64) ([]int64, error)
	EmitChildEventToGuardians(context.Context, int64, int64, []int64, ChildEvent) error
	ResolveDecisionAudience(context.Context, int64, int64, []int64) (DecisionAudience, error)
	EmitDecisionAudience(context.Context, int64, int64, DecisionAudience, ChildEvent, ChildEvent) error
}
