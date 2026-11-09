package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedBelegRetentionStep records, as the moto team in the operator portal,
// the instruction of the seeded Träger for its holiday invoices and a
// suspension of its Jahresbescheide (#4720, ADR 0184), so the operator view
// shows an instruction, a suspension and, after the Worker's first
// evaluation, the clarification list of the other Beleg types. Eight years
// after the issue year keep every seeded Beleg; nothing goes.
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
		"beleg_type": "holiday_document", "trigger_event": "issued", "period": 8,
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
	fmt.Println("  Beleg retention: instruction for holiday invoices and a suspension of the Jahresbescheide recorded")
	return nil
}
