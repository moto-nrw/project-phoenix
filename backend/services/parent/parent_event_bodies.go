package parent

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
)

// emitSelfServicePill posts the chat pill mirroring a parent self-service
// action (sick note, one-day pickup change). It MUST be scheduled from a
// tenant.RegisterAfterCommit callback: the emitter opens its own detached
// tenant transaction and is best-effort, so a pill failure can never roll
// back the sick note / pickup change (nor vice versa).
func (s *service) emitSelfServicePill(tenantID, studentID, accountID int64, eventType, body, refTable string, refID *int64) {
	if s.Emitter == nil {
		return
	}
	s.Emitter.EmitChildEvent(tenantID, studentID, accountID, parentmessaging.ChildEvent{
		EventType:      eventType,
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: accountID,
		Body:           body,
		RefTable:       refTable,
		RefID:          refID,
	})
}

// wakeChildGuardians fans a message-INDEPENDENT parent_child_updated SSE event
// to EVERY guardian of the child, so a SECOND guardian's already-open parents-
// app tab refetches the child's care state after THIS guardian's self-service
// write (sick note, care exception). emitSelfServicePill only appends a pill to
// the ACTING guardian's own thread and wakes that guardian; a co-guardian would
// otherwise keep showing a stale "Heute" pickup time or presence until they
// refocus or reload (#1725 review). Like emitSelfServicePill it MUST be
// scheduled from a tenant.RegisterAfterCommit callback: BroadcastChildUpdate-
// ToGuardians opens its own detached tenant transaction to read the guardian
// list, so a woken client never reads the pre-commit snapshot. A nil emitter is
// a safe no-op.
func (s *service) wakeChildGuardians(tenantID, studentID int64) {
	if s.Emitter == nil {
		return
	}
	s.Emitter.BroadcastChildUpdateToGuardians(tenantID, studentID)
}

func firstStatusID(rows []*activeModels.StudentStatusDay) *int64 {
	for _, row := range rows {
		if row != nil && row.ID > 0 {
			id := row.ID
			return &id
		}
	}
	return nil
}

// sickNoteEventBody renders the chat pill for a parent absence submission. The
// label MUST follow the submitted status: a "Krankmeldung" (sick) and an
// "Entschuldigte Abwesenheit" (excused — e.g. an appointment, #1735) are
// distinct in the data and to staff, so an excused absence must not be
// recorded as a sickness.
func sickNoteEventBody(status string, dates []timezone.Date) string {
	parts := make([]string, 0, len(dates))
	for _, d := range dates {
		parts = append(parts, d.Format("02.01."))
	}
	label := "Krankmeldung"
	if status == activeModels.StudentStatusDayExcused {
		label = "Entschuldigte Abwesenheit"
	}
	return label + ": " + strings.Join(parts, ", ")
}

// careExceptionEventBody renders the chat pill for a guardian care-time
// submission. The headline stays neutral ("Betreuungszeit") because either leg
// (arrival, pickup, or both) may have changed — labelling it as an "Abholung"
// change would record a pickup change that did not happen on an arrival-only
// submit. The appended value lines name the concrete leg(s) that were set.
func careExceptionEventBody(date timezone.Date, pickupTime, arrivalTime *time.Time) string {
	parts := []string{"Betreuungszeit " + date.Format("02.01.") + " geändert"}
	if arrivalTime != nil {
		parts = append(parts, "Ankunft "+arrivalTime.Format("15:04"))
	}
	if pickupTime != nil {
		parts = append(parts, "Abholung "+pickupTime.Format("15:04"))
	}
	return strings.Join(parts, ": ")
}

// careExceptionRef resolves the pill's backreference to the exception row the
// submission wrote (pickup preferred, else arrival). Best-effort: a lookup
// failure just leaves the pill without a ref.
func (s *service) careExceptionRef(ctx context.Context, studentID int64, date timezone.Date) (string, *int64) {
	pickup, err := s.PickupExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
	if err == nil && pickup != nil {
		id := pickup.ID
		return "schedule.student_pickup_exceptions", &id
	}
	arrival, err := s.ArrivalExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
	if err == nil && arrival != nil {
		id := arrival.ID
		return "schedule.student_arrival_exceptions", &id
	}
	return "", nil
}
