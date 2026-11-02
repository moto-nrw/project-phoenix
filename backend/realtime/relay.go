package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The audiences a relayed signal can address. A notification can only use
// the ones its sending channel uses: tenant, admins, group, staff accounts,
// school accounts and guardian.
const (
	AudienceGroup             = "group"
	audienceGroups            = "groups"
	AudienceTenant            = "tenant"
	AudienceAdmins            = "admins"
	AudienceStaffAccounts     = "staff_accounts"
	AudienceSchoolAccounts    = "school_accounts"
	audienceParentMessage     = "parent_message"
	audienceLeadershipMessage = "leadership_message"
	AudienceGuardian          = "guardian"
)

// notificationLoadTimeout bounds the content lookup of one relayed
// notification; the listener handles signals one after another.
const notificationLoadTimeout = 2 * time.Second

// NotificationAddress is the school-internal address of a relayed
// notification signal: an audience with its group, accounts or guardian.
type NotificationAddress struct {
	Audience   string
	Topic      string
	AccountIDs []int64
	GuardianID int64
}

// NotificationContent loads a notification another process relayed by ID
// (#5504). It reads the notification in the school's transaction and returns
// the event to show and the part of address the stored notification really
// names. ok is false for an unknown, expired, foreign or differently
// addressed ID: then nothing is shown. Content never comes from the signal.
type NotificationContent func(ctx context.Context, schoolID, notificationID int64, address NotificationAddress) (event Event, permitted NotificationAddress, ok bool, err error)

// errNotificationContentUnbound reports a relayed notification in a process
// whose owner has not bound the content lookup.
var errNotificationContentUnbound = errors.New("realtime notification content is not bound")

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

// RelayNotifications binds the content lookup of relayed notifications and
// reports whether this hub relays them at all. Only a hub that publishes to
// other processes relays notifications; the owner binds it once per process,
// and until then such signals are discarded.
func (h *Hub) RelayNotifications(load NotificationContent) bool {
	if h.publish == nil || load == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.notificationContent = load
	return true
}

// normalize rejects unscoped or unknown audiences rather than broadening them
// and reduces every ID and topic to its canonical form. Invalid optional IDs
// are dropped; an address without a valid target is rejected.
func (s *signal) normalize() error {
	if s.SchoolID <= 0 || s.Origin == "" || (!relayEventType(s.Type) && s.Type != EventNotification) {
		return fmt.Errorf("invalid realtime school or event")
	}
	s.ActiveGroupID = NormalizeTopic(s.ActiveGroupID)
	s.Topic = NormalizeTopic(s.Topic)
	s.Topics = normalizeTopics(s.Topics)
	s.AccountIDs = positiveIDs(s.AccountIDs)
	if err := s.validateNotification(); err != nil {
		return err
	}
	switch s.Audience {
	case AudienceTenant, AudienceAdmins:
	case AudienceGroup:
		if s.Topic == "" {
			return fmt.Errorf("realtime group is required")
		}
	case audienceGroups:
		if len(s.Topics) == 0 {
			return fmt.Errorf("realtime groups are required")
		}
	case AudienceStaffAccounts, AudienceSchoolAccounts:
		if len(s.AccountIDs) == 0 {
			return fmt.Errorf("realtime accounts are required")
		}
	case audienceParentMessage, audienceLeadershipMessage, AudienceGuardian:
		if s.GuardianID <= 0 {
			return fmt.Errorf("realtime guardian is required")
		}
	default:
		return fmt.Errorf("unknown realtime audience")
	}
	return nil
}

// validateNotification checks the narrower ID-only notification contract.
func (s signal) validateNotification() error {
	if s.Type != EventNotification {
		return nil
	}
	if numericID(s.Data.NotificationID) == nil {
		return fmt.Errorf("realtime notification ID is required")
	}
	switch s.Audience {
	case audienceGroups, audienceParentMessage, audienceLeadershipMessage:
		return fmt.Errorf("realtime notification audience is not supported")
	}
	return nil
}

func (s signal) address() NotificationAddress {
	return NotificationAddress{Audience: s.Audience, Topic: s.Topic, AccountIDs: s.AccountIDs, GuardianID: s.GuardianID}
}

// ReceiveSignal dispatches a validated transport signal to this API's clients.
func (h *Hub) ReceiveSignal(payload string) error {
	var s signal
	if err := json.Unmarshal([]byte(payload), &s); err != nil {
		return fmt.Errorf("invalid realtime envelope")
	}
	if err := s.normalize(); err != nil {
		return err
	}
	if s.Origin == h.origin {
		return nil
	}
	if s.Type == EventNotification {
		return h.receiveNotification(s)
	}
	event := NewEvent(s.Type, s.ActiveGroupID, s.Data.eventData(s.Type))
	return h.dispatch(s, event)
}

// receiveNotification shows a relayed notification only as its owner loads
// it: from the school's own row, to the recipients that row names.
func (h *Hub) receiveNotification(s signal) error {
	h.mu.RLock()
	load := h.notificationContent
	h.mu.RUnlock()
	if load == nil {
		return errNotificationContentUnbound
	}
	id, err := strconv.ParseInt(*numericID(s.Data.NotificationID), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid realtime notification ID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), notificationLoadTimeout)
	defer cancel()
	event, permitted, ok, err := load(ctx, s.SchoolID, id, s.address())
	if err != nil {
		return fmt.Errorf("load relayed notification: %w", err)
	}
	if !ok {
		return nil
	}
	if event.Type != EventNotification || !narrows(s.address(), permitted) {
		return fmt.Errorf("relayed notification content widened its address")
	}
	s.Topic, s.AccountIDs, s.GuardianID = permitted.Topic, permitted.AccountIDs, permitted.GuardianID
	return h.dispatch(s, event)
}

// narrows reports whether permitted addresses at most what the signal named.
func narrows(signal, permitted NotificationAddress) bool {
	if signal.Audience != permitted.Audience || signal.Topic != permitted.Topic || signal.GuardianID != permitted.GuardianID {
		return false
	}
	for _, accountID := range permitted.AccountIDs {
		if !slices.Contains(signal.AccountIDs, accountID) {
			return false
		}
	}
	return true
}

func (h *Hub) dispatch(s signal, event Event) error {
	switch s.Audience {
	case AudienceGroup:
		return h.localBroadcastToGroup(s.SchoolID, s.Topic, event)
	case audienceGroups:
		return h.localBroadcastToGroups(s.SchoolID, s.Topics, event)
	case AudienceTenant:
		return h.localBroadcastToTenant(s.SchoolID, event)
	case AudienceAdmins:
		return h.localBroadcastToTenantAdmins(s.SchoolID, event)
	case AudienceStaffAccounts:
		return h.localBroadcastToStaffAccounts(s.SchoolID, s.AccountIDs, event)
	case AudienceSchoolAccounts:
		return h.localBroadcastToSchoolAccounts(s.SchoolID, s.AccountIDs, event)
	case audienceParentMessage:
		return h.localBroadcastParentMessage(s.SchoolID, s.GuardianID, event)
	case audienceLeadershipMessage:
		return h.localBroadcastLeadershipParentMessage(s.SchoolID, s.GuardianID, event)
	case AudienceGuardian:
		return h.localBroadcastToGuardian(s.SchoolID, s.GuardianID, event)
	}
	return fmt.Errorf("unknown realtime audience")
}

func (h *Hub) BroadcastToGroup(tenantID int64, activeGroupID string, event Event) error {
	if err := h.localBroadcastToGroup(tenantID, activeGroupID, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: AudienceGroup, Topic: activeGroupID}, event)
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
		return h.publishSignal(signal{SchoolID: tenantID, Audience: AudienceTenant}, event)
	}
	return nil
}

func (h *Hub) BroadcastToTenantAdmins(tenantID int64, event Event) error {
	if err := h.localBroadcastToTenantAdmins(tenantID, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: AudienceAdmins}, event)
	}
	return nil
}

func (h *Hub) BroadcastToStaffAccounts(tenantID int64, accountIDs []int64, event Event) error {
	if err := h.localBroadcastToStaffAccounts(tenantID, accountIDs, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: AudienceStaffAccounts, AccountIDs: accountIDs}, event)
	}
	return nil
}

func (h *Hub) BroadcastToSchoolAccounts(tenantID int64, accountIDs []int64, event Event) error {
	if err := h.localBroadcastToSchoolAccounts(tenantID, accountIDs, event); err != nil {
		return err
	}
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: AudienceSchoolAccounts, AccountIDs: accountIDs}, event)
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
		return h.publishSignal(signal{SchoolID: tenantID, Audience: AudienceGuardian, GuardianID: guardianAccountID}, event)
	}
	return nil
}

func (h *Hub) publishSignal(signal signal, event Event) error {
	switch {
	case event.Type == EventNotification:
		// Only a notification its owner stored can be loaded elsewhere; the
		// signal carries its ID, never title, text or link (#5504).
		if event.notificationID <= 0 {
			return nil
		}
		id := strconv.FormatInt(event.notificationID, 10)
		signal.Data = invalidationData{NotificationID: &id}
	case relayEventType(event.Type):
		signal.Data = invalidationFromEvent(event.Type, event.Data)
	default:
		return nil
	}
	signal.Origin = h.origin
	signal.Type = event.Type
	signal.ActiveGroupID = event.ActiveGroupID
	if err := signal.normalize(); err != nil {
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
	Source     *string   `json:"source,omitempty"`
	// NotificationID names a stored notification; only notification signals
	// carry it, and nothing else.
	NotificationID *string `json:"notification_id,omitempty"`
}

func invalidationFromEvent(kind EventType, data EventData) invalidationData {
	return invalidationData{
		StudentID: numericID(data.StudentID), StudentIDs: numericIDs(data.StudentIDs),
		GroupIDs: numericIDs(data.GroupIDs), InstanceID: numericID(data.InstanceID),
		ThreadID: numericID(data.ThreadID), Reason: invalidationReason(data.Reason),
		Source: staffMessageSource(kind, data.Source),
	}
}

func (data invalidationData) eventData(kind EventType) EventData {
	return EventData{
		StudentID: numericID(data.StudentID), StudentIDs: numericIDs(data.StudentIDs),
		GroupIDs: numericIDs(data.GroupIDs), InstanceID: numericID(data.InstanceID),
		ThreadID: numericID(data.ThreadID), Reason: invalidationReason(data.Reason),
		Source: staffMessageSource(kind, data.Source),
	}
}

// Only staff messages need the numeric sender for read-cursor suppression.
// Other sources remain local; arbitrary source text is never relayed.
func staffMessageSource(kind EventType, source *string) *string {
	if kind != EventStaffMessage {
		return nil
	}
	return numericID(source)
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

// positiveIDs drops non-positive and repeated account IDs.
func positiveIDs(ids []int64) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !slices.Contains(result, id) {
			result = append(result, id)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// NormalizeTopic accepts the two topic shapes producers use, an active
// group ID and EducationGroupTopic, and returns "" for anything else.
func NormalizeTopic(topic string) string {
	if id, ok := strings.CutPrefix(topic, "edu:"); ok {
		if normalized := numericID(&id); normalized != nil {
			return "edu:" + *normalized
		}
		return ""
	}
	if normalized := numericID(&topic); normalized != nil {
		return *normalized
	}
	return ""
}

func normalizeTopics(topics []string) []string {
	result := make([]string, 0, len(topics))
	for _, topic := range topics {
		if normalized := NormalizeTopic(topic); normalized != "" && !slices.Contains(result, normalized) {
			result = append(result, normalized)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
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
