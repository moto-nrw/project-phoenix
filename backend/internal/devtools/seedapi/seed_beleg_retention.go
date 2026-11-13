package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// seedBelegRetentionStep records, as the moto team in the operator portal,
// the instruction of the seeded Träger for its holiday invoices and a
// suspension of its Jahresbescheide (#4720, ADR 0184), so the operator view
// shows an instruction, a suspension and the clarification list of the
// other Beleg types, whose first entry carries a review. Eight years after
// the issue year keep every seeded Beleg; nothing goes.
type seedBelegRetentionStep struct{}

func (seedBelegRetentionStep) Name() string { return "Seeding the Beleg retention" }

func (seedBelegRetentionStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.OrganizationID <= 0 {
		return fmt.Errorf("the Beleg retention seed prerequisites are not available")
	}
	previousAuth := rt.Client.auth
	rt.Client.BindAuth(rt.OperatorAuth)
	defer rt.Client.BindAuth(previousAuth)
	today := calendar.TodayDate()
	base := fmt.Sprintf("/operator/organizations/%d/beleg-retention", rt.Bootstrap.OrganizationID)
	if _, err := rt.Client.Post(base+"/instructions", map[string]any{
		"beleg_type": "holiday_document", "trigger_event": "issued", "period": 8, "basis": "booking_voucher",
		"legal_basis": "§ 147 AO, § 14b UStG", "instructed_by": "Kämmerei des Trägers",
		"instructed_on": today.String(), "instruction_reference": "Demo-Weisung zur Belegaufbewahrung",
	}); err != nil {
		return fmt.Errorf("record the Beleg retention instruction: %w", err)
	}
	if _, err := rt.Client.Post(base+"/suspensions", map[string]any{
		"beleg_type": "care_notice", "reason": "Widerspruchsverfahren offen", "scope": "alle Jahresbescheide",
		"responsible": "Kämmerei des Trägers", "review_on": today.AddDays(90).String(),
	}); err != nil {
		return fmt.Errorf("record the Beleg retention suspension: %w", err)
	}
	// The instruction evaluated the Träger, so the other types wait on the
	// clarification list; the moto team reviewed the first entry.
	raw, err := rt.Client.Get(base)
	if err != nil {
		return fmt.Errorf("read the Beleg retention: %w", err)
	}
	var overview struct {
		Data struct {
			Clarifications []struct {
				BelegType string `json:"beleg_type"`
				Reason    string `json:"reason"`
			} `json:"clarifications"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &overview); err != nil {
		return fmt.Errorf("decode the Beleg retention: %w", err)
	}
	if len(overview.Data.Clarifications) == 0 {
		// A Träger without a Beleg has nothing to clarify.
		fmt.Println("  Beleg retention: instruction for holiday invoices and a suspension of the Jahresbescheide recorded")
		return nil
	}
	entry := overview.Data.Clarifications[0]
	if _, err := rt.Client.Post(base+"/clarification-reviews", map[string]any{
		"beleg_type": entry.BelegType, "reason": entry.Reason,
		"note": "Träger um Weisung gebeten, Rückmeldung der Kämmerei steht aus", "escalated": true,
		"escalation_reason": "Keine Rückmeldung innerhalb von 30 Tagen", "next_review_on": today.AddDays(14).String(),
	}); err != nil {
		return fmt.Errorf("record the Beleg clarification review: %w", err)
	}
	fmt.Println("  Beleg retention: instruction for holiday invoices, a suspension of the Jahresbescheide and a clarification review recorded")
	return nil
}
