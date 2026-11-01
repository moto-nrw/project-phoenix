package realtime

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strconv"
)

const (
	audienceGroup             = "group"
	audienceGroups            = "groups"
	audienceTenant            = "tenant"
	audienceAdmins            = "admins"
	audienceStaffAccounts     = "staff_accounts"
	audienceSchoolAccounts    = "school_accounts"
	audienceParentMessage     = "parent_message"
	audienceLeadershipMessage = "leadership_message"
	audienceGuardian          = "guardian"
)

// signal retains the original audience alongside its school. It is an internal
// process-to-process envelope, not the browser's SSE payload.
type signal struct {
	SchoolID      int64            `json:"school_id"`
	Audience      string           `json:"audience"`
	Topic         string           `json:"topic,omitempty"`
	Topics        []string         `json:"topics,omitempty"`
	AccountIDs    []int64          `json:"account_ids,omitempty"`
	GuardianID    int64            `json:"guardian_id,omitempty"`
	Origin        string           `json:"origin"`
	Type          EventType        `json:"type"`
	ActiveGroupID string           `json:"active_group_id,omitempty"`
	Data          invalidationData `json:"ids"`
}

// NewRelayedHub delivers locally and also publishes invalidation signals.
// ReceiveSignal performs local fan-out only, so received messages never loop.
func NewRelayedHub(logger *slog.Logger, publish func(string) error, observers ...Observers) (*Hub, error) {
	if publish == nil {
		return nil, fmt.Errorf("realtime publisher is required")
	}
	hub := NewHub(logger, observers...)
	hub.publish = publish
	// Origin is echo suppression, not authentication. The channel is untrusted.
	hub.origin = fmt.Sprintf("%016x%016x", rand.Uint64(), rand.Uint64())
	return hub, nil
}

// Validate rejects unscoped or unknown audiences rather than broadening them.
func (s signal) Validate() error {
	if s.SchoolID <= 0 || !relayEventType(s.Type) || s.Origin == "" {
		return fmt.Errorf("invalid realtime school or event")
	}
	switch s.Audience {
	case audienceTenant, audienceAdmins:
	case audienceGroup:
		if s.Topic == "" {
			return fmt.Errorf("realtime group is required")
		}
	case audienceGroups:
		if len(s.Topics) == 0 {
			return fmt.Errorf("realtime groups are required")
		}
	case audienceStaffAccounts, audienceSchoolAccounts:
		if len(s.AccountIDs) == 0 {
			return fmt.Errorf("realtime accounts are required")
		}
	case audienceParentMessage, audienceLeadershipMessage, audienceGuardian:
		if s.GuardianID <= 0 {
			return fmt.Errorf("realtime guardian is required")
		}
	default:
		return fmt.Errorf("unknown realtime audience")
	}
	return nil
}

// ReceiveSignal dispatches a validated transport signal to this API's clients.
func (h *Hub) ReceiveSignal(payload string) error {
	var s signal
	if err := json.Unmarshal([]byte(payload), &s); err != nil {
		return fmt.Errorf("invalid realtime envelope")
	}
	if err := s.Validate(); err != nil {
		return err
	}
	if s.Origin == h.origin {
		return nil
	}
	event := NewEvent(s.Type, s.ActiveGroupID, s.Data.eventData())
	switch s.Audience {
	case audienceGroup:
		return h.localBroadcastToGroup(s.SchoolID, s.Topic, event)
	case audienceGroups:
		return h.localBroadcastToGroups(s.SchoolID, s.Topics, event)
	case audienceTenant:
		return h.localBroadcastToTenant(s.SchoolID, event)
	case audienceAdmins:
		return h.localBroadcastToTenantAdmins(s.SchoolID, event)
	case audienceStaffAccounts:
		return h.localBroadcastToStaffAccounts(s.SchoolID, s.AccountIDs, event)
	case audienceSchoolAccounts:
		return h.localBroadcastToSchoolAccounts(s.SchoolID, s.AccountIDs, event)
	case audienceParentMessage:
		return h.localBroadcastParentMessage(s.SchoolID, s.GuardianID, event)
	case audienceLeadershipMessage:
		return h.localBroadcastLeadershipParentMessage(s.SchoolID, s.GuardianID, event)
	case audienceGuardian:
		return h.localBroadcastToGuardian(s.SchoolID, s.GuardianID, event)
	}
	return fmt.Errorf("unknown realtime audience")
}

func (h *Hub) BroadcastToGroup(tenantID int64, activeGroupID string, event Event) error {
	if err := h.localBroadcastToGroup(tenantID, activeGroupID, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: audienceGroup, Topic: activeGroupID}, event)
	}
	return nil
}

func (h *Hub) BroadcastToGroups(tenantID int64, topics []string, event Event) error {
	if err := h.localBroadcastToGroups(tenantID, topics, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: audienceGroups, Topics: topics}, event)
	}
	return nil
}

func (h *Hub) BroadcastToTenant(tenantID int64, event Event) error {
	if err := h.localBroadcastToTenant(tenantID, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: audienceTenant}, event)
	}
	return nil
}

func (h *Hub) BroadcastToTenantAdmins(tenantID int64, event Event) error {
	if err := h.localBroadcastToTenantAdmins(tenantID, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: audienceAdmins}, event)
	}
	return nil
}

func (h *Hub) BroadcastToStaffAccounts(tenantID int64, accountIDs []int64, event Event) error {
	if err := h.localBroadcastToStaffAccounts(tenantID, accountIDs, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: audienceStaffAccounts, AccountIDs: accountIDs}, event)
	}
	return nil
}

func (h *Hub) BroadcastToSchoolAccounts(tenantID int64, accountIDs []int64, event Event) error {
	if err := h.localBroadcastToSchoolAccounts(tenantID, accountIDs, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: audienceSchoolAccounts, AccountIDs: accountIDs}, event)
	}
	return nil
}

func (h *Hub) BroadcastParentMessage(tenantID, guardianAccountID int64, event Event) error {
	if err := h.localBroadcastParentMessage(tenantID, guardianAccountID, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: audienceParentMessage, GuardianID: guardianAccountID}, event)
	}
	return nil
}

func (h *Hub) BroadcastLeadershipParentMessage(tenantID, guardianAccountID int64, event Event) error {
	if err := h.localBroadcastLeadershipParentMessage(tenantID, guardianAccountID, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: audienceLeadershipMessage, GuardianID: guardianAccountID}, event)
	}
	return nil
}

func (h *Hub) BroadcastToGuardian(tenantID, guardianAccountID int64, event Event) error {
	if err := h.localBroadcastToGuardian(tenantID, guardianAccountID, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: audienceGuardian, GuardianID: guardianAccountID}, event)
	}
	return nil
}

func (h *Hub) publishSignal(signal signal, event Event) error {
	if !relayEventType(event.Type) {
		return nil
	}
	signal.Origin = h.origin
	signal.Type = event.Type
	signal.ActiveGroupID = event.ActiveGroupID
	signal.Data = invalidationFromEvent(event.Data)
	if err := signal.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("encode realtime signal: %w", err)
	}
	if len(payload) >= 8000 {
		return fmt.Errorf("realtime signal exceeds PostgreSQL NOTIFY limit")
	}
	return h.publish(string(payload))
}

// invalidationData is deliberately separate from EventData: adding display
// fields to an SSE event must never expose them on the unauthenticated bus.
// The receiver reconstructs the event through the same allowlist.
type invalidationData struct {
	StudentID  *string   `json:"student_id,omitempty"`
	StudentIDs *[]string `json:"student_ids,omitempty"`
	GroupIDs   *[]string `json:"group_ids,omitempty"`
	InstanceID *string   `json:"instance_id,omitempty"`
	ThreadID   *string   `json:"thread_id,omitempty"`
	Reason     *string   `json:"reason,omitempty"`
}

func invalidationFromEvent(data EventData) invalidationData {
	return invalidationData{
		StudentID: numericID(data.StudentID), StudentIDs: numericIDs(data.StudentIDs),
		GroupIDs: numericIDs(data.GroupIDs), InstanceID: numericID(data.InstanceID),
		ThreadID: numericID(data.ThreadID), Reason: invalidationReason(data.Reason),
	}
}

func (data invalidationData) eventData() EventData {
	return EventData{
		StudentID: numericID(data.StudentID), StudentIDs: numericIDs(data.StudentIDs),
		GroupIDs: numericIDs(data.GroupIDs), InstanceID: numericID(data.InstanceID),
		ThreadID: numericID(data.ThreadID), Reason: invalidationReason(data.Reason),
	}
}

func numericID(id *string) *string {
	if id == nil {
		return nil
	}
	value, err := strconv.ParseInt(*id, 10, 64)
	if err != nil || value <= 0 {
		return nil
	}
	normalized := strconv.FormatInt(value, 10)
	return &normalized
}

func numericIDs(ids *[]string) *[]string {
	if ids == nil {
		return nil
	}
	result := make([]string, 0, len(*ids))
	for _, id := range *ids {
		if value := numericID(&id); value != nil {
			result = append(result, *value)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return &result
}

// Only fixed invalidation reasons influence cache selection. No free text.
func invalidationReason(reason *string) *string {
	if reason == nil {
		return nil
	}
	switch *reason {
	case "student_moved", "activity_started", "activity_ended", "instance_started", "instance_completed", "instance_cancelled", "instance_overdue", "additional_supervisor_assigned", "supervision_handed_over", "supervision_released", "supervisors_changed", "timetable_attendance_updated":
		return reason
	default:
		return nil
	}
}

func relayEventType(kind EventType) bool {
	switch kind {
	case EventStudentCheckIn, EventStudentCheckOut, EventStudentUpdated, EventStudentCompanionsChanged, EventBulkStudentCheckOut, EventBulkStudentCheckIn, EventActivityStart, EventActivityEnd, EventActivityUpdate, EventInstanceStarted, EventInstanceCompleted, EventInstanceCancelled, EventInstanceOverdue, EventStaffingDeviationChanged, EventDashboardCountsChanged, EventGroupAccessChanged, EventStaffTimeTrackingChanged, EventActiveSupervisionChanged, EventArrivalScheduleChanged, EventPickupScheduleChanged, EventChangeRequestsChanged, EventTenantSettingsChanged, EventParentMessage, EventParentMessageRead, EventParentMessageUnreadChanged, EventStaffMessage, EventParentChildUpdated, EventClassDayChanged:
		return true
	default:
		return false
	}
}
