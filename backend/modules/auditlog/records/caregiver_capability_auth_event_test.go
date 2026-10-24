package records

import "testing"

func TestNewCaregiverCapabilityAuthEvent(t *testing.T) {
	t.Parallel()

	accountID := int64(4082)
	for _, tc := range []struct {
		name      string
		enabled   bool
		eventType string
	}{
		{name: "enabled", enabled: true, eventType: EventTypeCaregiverCapabilityEnabled},
		{name: "disabled", enabled: false, eventType: EventTypeCaregiverCapabilityDisabled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			event := NewCaregiverCapabilityAuthEvent(accountID, tc.enabled)
			if event.AccountID != accountID || event.EventType != tc.eventType || !event.Success || event.IPAddress != "0.0.0.0" {
				t.Fatalf("unexpected event: %+v", event)
			}
			if err := event.Validate(); err != nil {
				t.Fatalf("Validate() = %v", err)
			}
		})
	}
}
