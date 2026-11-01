package realtime

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// signal retains the original audience alongside its school. It is an internal
// process-to-process envelope, not the browser's SSE payload.
type signal struct {
	SchoolID   int64    `json:"school_id"`
	Audience   string   `json:"audience"`
	Topic      string   `json:"topic,omitempty"`
	Topics     []string `json:"topics,omitempty"`
	AccountIDs []int64  `json:"account_ids,omitempty"`
	GuardianID int64    `json:"guardian_id,omitempty"`
	Event      Event    `json:"event"`
}

// NewRelayedHub sends broadcasts through the supplied transport. ReceiveSignal
// performs local fan-out only, so a received message can never form a loop.
func NewRelayedHub(logger *slog.Logger, publish func(string) error, observers ...Observers) (*Hub, error) {
	if publish == nil {
		return nil, fmt.Errorf("realtime publisher is required")
	}
	hub := NewHub(logger, observers...)
	hub.publish = publish
	return hub, nil
}

// Validate rejects unscoped or unknown audiences rather than broadening them.
func (s signal) Validate() error {
	if s.SchoolID <= 0 || s.Event.Type == "" {
		return fmt.Errorf("invalid realtime school or event")
	}
	switch s.Audience {
	case "tenant", "admins":
	case "group":
		if s.Topic == "" {
			return fmt.Errorf("realtime group is required")
		}
	case "groups":
		if len(s.Topics) == 0 {
			return fmt.Errorf("realtime groups are required")
		}
	case "staff_accounts", "school_accounts":
		if len(s.AccountIDs) == 0 {
			return fmt.Errorf("realtime accounts are required")
		}
	case "parent_message", "leadership_message", "guardian":
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
	switch s.Audience {
	case "group":
		return h.localBroadcastToGroup(s.SchoolID, s.Topic, s.Event)
	case "groups":
		return h.localBroadcastToGroups(s.SchoolID, s.Topics, s.Event)
	case "tenant":
		return h.localBroadcastToTenant(s.SchoolID, s.Event)
	case "admins":
		return h.localBroadcastToTenantAdmins(s.SchoolID, s.Event)
	case "staff_accounts":
		return h.localBroadcastToStaffAccounts(s.SchoolID, s.AccountIDs, s.Event)
	case "school_accounts":
		return h.localBroadcastToSchoolAccounts(s.SchoolID, s.AccountIDs, s.Event)
	case "parent_message":
		return h.localBroadcastParentMessage(s.SchoolID, s.GuardianID, s.Event)
	case "leadership_message":
		return h.localBroadcastLeadershipParentMessage(s.SchoolID, s.GuardianID, s.Event)
	case "guardian":
		return h.localBroadcastToGuardian(s.SchoolID, s.GuardianID, s.Event)
	}
	return fmt.Errorf("unknown realtime audience")
}

func (h *Hub) BroadcastToGroup(tenantID int64, activeGroupID string, event Event) error {
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: "group", Event: event, Topic: activeGroupID})
	}
	return h.localBroadcastToGroup(tenantID, activeGroupID, event)
}

func (h *Hub) BroadcastToGroups(tenantID int64, topics []string, event Event) error {
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: "groups", Event: event, Topics: topics})
	}
	return h.localBroadcastToGroups(tenantID, topics, event)
}

func (h *Hub) BroadcastToTenant(tenantID int64, event Event) error {
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: "tenant", Event: event})
	}
	return h.localBroadcastToTenant(tenantID, event)
}

func (h *Hub) BroadcastToTenantAdmins(tenantID int64, event Event) error {
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: "admins", Event: event})
	}
	return h.localBroadcastToTenantAdmins(tenantID, event)
}

func (h *Hub) BroadcastToStaffAccounts(tenantID int64, accountIDs []int64, event Event) error {
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: "staff_accounts", Event: event, AccountIDs: accountIDs})
	}
	return h.localBroadcastToStaffAccounts(tenantID, accountIDs, event)
}

func (h *Hub) BroadcastToSchoolAccounts(tenantID int64, accountIDs []int64, event Event) error {
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: "school_accounts", Event: event, AccountIDs: accountIDs})
	}
	return h.localBroadcastToSchoolAccounts(tenantID, accountIDs, event)
}

func (h *Hub) BroadcastParentMessage(tenantID, guardianAccountID int64, event Event) error {
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: "parent_message", Event: event, GuardianID: guardianAccountID})
	}
	return h.localBroadcastParentMessage(tenantID, guardianAccountID, event)
}

func (h *Hub) BroadcastLeadershipParentMessage(tenantID, guardianAccountID int64, event Event) error {
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: "leadership_message", Event: event, GuardianID: guardianAccountID})
	}
	return h.localBroadcastLeadershipParentMessage(tenantID, guardianAccountID, event)
}

func (h *Hub) BroadcastToGuardian(tenantID, guardianAccountID int64, event Event) error {
	if h.publish != nil {
		return h.publishSignal(signal{SchoolID: tenantID, Audience: "guardian", Event: event, GuardianID: guardianAccountID})
	}
	return h.localBroadcastToGuardian(tenantID, guardianAccountID, event)
}

func (h *Hub) publishSignal(signal signal) error {
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
