package users

const (
	ParentMessageKindMessage = "message"
	ParentMessageKindEvent   = "event"
	ParentMessageKindRequest = "request"

	ParentMessageRequestStatusOpen      = "offen"
	ParentMessageRequestStatusDone      = "erledigt"
	ParentMessageRequestStatusRejected  = "abgelehnt"
	ParentMessageRequestStatusWithdrawn = "zurueckgezogen"
	ParentMessageRequestCareSchedule    = "care_schedule"
	// ParentMessageRequestPickupChange tags pills for a SINGLE-DAY pickup time
	// change (schedule.care_schedule_change_requests with
	// request_kind='pickup_change'). Distinct from care_schedule, which is the
	// permanent weekly plan: the localized parents portal and the decision
	// push both render from this token, so sharing it would announce a
	// one-day change as a permanent one.
	ParentMessageRequestPickupChange = "pickup_change"
	// ParentMessageRequestMasterData tags notification pills for Stammdaten
	// change requests (users.student_data_change_requests). Unlike
	// care_schedule it was never a chat-request kind — it only ever appears on
	// kind='event' rows.
	ParentMessageRequestMasterData = "master_data"
	// ParentMessageRequestExcusedAbsence tags notification pills for excused
	// absence approval requests (active.excused_absence_requests, #1845). Like
	// master_data it only ever appears on kind='event' rows, never as a chat
	// request.
	ParentMessageRequestExcusedAbsence = "excused_absence"
	// ParentMessageRequestSickAbsence distinguishes a gated Krankmeldung from an
	// excused absence in localized parent timeline events and decision pushes.
	ParentMessageRequestSickAbsence = "sick_absence"

	// Event types for kind='event' system pills that mirror a change request's
	// lifecycle. ParentMessageEventRequestCreated marks a submitted request (its
	// actionable signal is the Änderungsanfragen queue badge, NOT the unread chat
	// count — see IsCounterpartMessage / counterpartUnread); ParentMessageEventRequestStatus
	// CLOSES it (done / rejected / withdrawn). Kept as constants so the model
	// helpers, the emitter's reconcile gate, and the SQL predicate stay aligned.
	ParentMessageEventRequestCreated = "request_created"
	ParentMessageEventRequestStatus  = "request_status"
)
