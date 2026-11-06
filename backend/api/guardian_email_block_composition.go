package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// errGuardianEmailBlockLiftFailed is a lift the provider did not complete. The
// address stays marked, so the school can try again.
var errGuardianEmailBlockLiftFailed = errors.New("the email block could not be lifted")

// GuardianEmailDelivery is Delivery as the guardian routes need it (#5934).
type GuardianEmailDelivery struct {
	Outcomes     deliveryRuntime.EmailOutcomes
	Suppressions deliveryRuntime.EmailSuppressions
	// Guardians finds the address on file; it answers ErrGuardianNotFound for a
	// guardian of another school.
	Guardians interface {
		FindGuardian(ctx context.Context, id int64) (peopledirectory.Guardian, error)
	}
}

// guardianEmailStates reads the state of a child's guardians' addresses in the
// request's tenant transaction.
func (d GuardianEmailDelivery) guardianEmailStates(ctx context.Context, addresses []string) (map[string]usersAPI.GuardianEmailState, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	marks, err := d.Outcomes.EmailAddressMarks(ctx, tenantID.Int64(), addresses)
	if err != nil {
		return nil, err
	}
	latest, err := d.Outcomes.LatestEmailOutcomes(ctx, tenantID.Int64(), []string{identityaccess.MailKindGuardianInvitation}, addresses)
	if err != nil {
		return nil, err
	}
	states := make(map[string]usersAPI.GuardianEmailState, len(addresses))
	for _, address := range addresses {
		key := deliveryContract.NormalizedEmailAddress(address)
		report := latest[key]
		states[key] = usersAPI.GuardianEmailState{
			MailOutcome: string(report.Outcome),
			AddressMark: string(marks[key]),
			SendBlock:   string(deliveryContract.EmailSendBlockOf(marks[key], report.Outcome)),
			OutcomeAt:   report.OutcomeAt,
		}
	}
	return states, nil
}

// guardianEmailBlockFlows binds the two writes to the address on file. The address
// never comes from the request, so only an address of this school can be changed;
// the guardian's lookup runs under the school's row-level security. The list is
// changed outside that transaction, because lifting waits for the provider.
func guardianEmailBlockFlows(delivery GuardianEmailDelivery, runtime peopleCompose.LegacyRepositoryRuntime) (stop, lift func(context.Context, int64) error) {
	addressOf := func(ctx context.Context, guardianID int64) (int64, string, error) {
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return 0, "", err
		}
		var address string
		err = runtime.InTenantTx(ctx, func(txCtx context.Context) error {
			guardian, findErr := delivery.Guardians.FindGuardian(txCtx, guardianID)
			if findErr != nil {
				return findErr
			}
			if guardian.Email != nil {
				address = strings.TrimSpace(*guardian.Email)
			}
			return nil
		})
		if err != nil {
			return 0, "", err
		}
		if address == "" {
			return 0, "", usersAPI.ErrGuardianHasNoEmail
		}
		return tenantID.Int64(), address, nil
	}
	stop = func(ctx context.Context, guardianID int64) error {
		tenantID, address, err := addressOf(ctx, guardianID)
		if err != nil {
			return err
		}
		return delivery.Suppressions.StopEmails(ctx, tenantID, address)
	}
	lift = func(ctx context.Context, guardianID int64) error {
		tenantID, address, err := addressOf(ctx, guardianID)
		if err != nil {
			return err
		}
		err = delivery.Suppressions.LiftEmailSuppression(ctx, tenantID, address)
		switch {
		case err == nil, errors.Is(err, deliveryContract.ErrEmailSuppressionNotFound):
			// Nothing holds the address as far as moto knows: it is free.
			return nil
		case errors.Is(err, deliveryContract.ErrEmailSuppressionNeedsReview),
			errors.Is(err, deliveryContract.ErrEmailSuppressionStaysAtProvider):
			return err
		default:
			return fmt.Errorf("%w: %w", errGuardianEmailBlockLiftFailed, err)
		}
	}
	return stop, lift
}
