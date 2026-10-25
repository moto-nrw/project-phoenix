package owners

import (
	"context"
	"errors"
	"fmt"
	"slices"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryModels "github.com/moto-nrw/project-phoenix/modules/delivery/subscriptions"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
)

// deliveryModuleGate decides per claimed intent whether the school still
// books the module the mail or push belongs to (#4895). Mail to parents and
// pushes to the parents portal belong to Eltern-App, enrollment mail to
// Online-Anmeldung, pushes to the school portal to Unterricht am Vormittag.
// Every other mail and push is Basis. The router of the notifications
// already drops new pushes; this gate also stops what was queued before the
// moto team cancelled the module.
type deliveryModuleGate struct {
	booking     *settingsValues.ModuleBooking
	mailModules map[string]settingsValues.ModuleKey
	basisMail   map[string]bool
}

// newDeliveryModuleGate classifies the mail kinds of the delivery worker:
// parentMailKinds belong to Eltern-App, enrollmentMailKinds to
// Online-Anmeldung, basisMailKinds to Basis.
func newDeliveryModuleGate(reader settingsValues.ModuleBookingReader, parentMailKinds, enrollmentMailKinds, basisMailKinds []string) (deliveryModuleGate, error) {
	booking, err := settingsValues.NewModuleBooking(reader)
	if err != nil {
		return deliveryModuleGate{}, fmt.Errorf("delivery module gate: %w", err)
	}
	modules := make(map[string]settingsValues.ModuleKey, len(parentMailKinds)+len(enrollmentMailKinds))
	for _, kind := range parentMailKinds {
		modules[kind] = settingsValues.ModuleElternApp
	}
	for _, kind := range enrollmentMailKinds {
		modules[kind] = settingsValues.ModuleOnlineAnmeldung
	}
	basis := make(map[string]bool, len(basisMailKinds))
	for _, kind := range basisMailKinds {
		basis[kind] = true
	}
	return deliveryModuleGate{booking: booking, mailModules: modules, basisMail: basis}, nil
}

// requireClassified refuses a worker with a mail kind the gate does not
// classify: a new kind must say whether it belongs to a module or to Basis,
// or a cancelled module would keep mailing through it.
func (g deliveryModuleGate) requireClassified(kinds []string) error {
	var unclassified []string
	for _, kind := range kinds {
		if _, module := g.mailModules[kind]; !module && !g.basisMail[kind] {
			unclassified = append(unclassified, kind)
		}
	}
	if len(unclassified) > 0 {
		slices.Sort(unclassified)
		return fmt.Errorf("delivery module gate: mail kinds without module or Basis: %v", unclassified)
	}
	return nil
}

// moduleOf returns the module of an intent; false for Basis.
func (g deliveryModuleGate) moduleOf(intent deliveryContract.ClaimedIntent) (settingsValues.ModuleKey, bool) {
	if intent.Transport == deliveryContract.TransportEmail {
		module, ok := g.mailModules[intent.Template]
		return module, ok
	}
	switch {
	case intent.Template == reminderPushKind, intent.PushRecipient.Portal == deliveryModels.PushPortalParent:
		return settingsValues.ModuleElternApp, true
	case intent.PushRecipient.Portal == deliveryModels.PushPortalSchool:
		return settingsValues.ModuleUnterrichtVormittag, true
	default:
		return "", false
	}
}

// deliverable is the provider's Deliverable seam.
func (g deliveryModuleGate) deliverable(ctx context.Context, intent deliveryContract.ClaimedIntent) (bool, error) {
	module, ok := g.moduleOf(intent)
	if !ok || intent.TenantID <= 0 {
		return true, nil
	}
	err := g.booking.RequireForTenant(ctx, intent.TenantID, module)
	var notBooked *settingsValues.ModuleNotBookedError
	if errors.As(err, &notBooked) {
		return false, nil
	}
	return err == nil, err
}
