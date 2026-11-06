package ports

import "context"

// CareContractReminders sends the due reminders of the care contracts that
// wait for the parents' signature and activates signed contracts whose
// effective date came, over every Träger (#4679). It opens its own units.
// The root binds the Betreuungsvertrag.
type CareContractReminders func(ctx context.Context) error
