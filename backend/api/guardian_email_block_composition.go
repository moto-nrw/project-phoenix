package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// errGuardianEmailBlockLiftFailed is a lift the provider did not complete. The
// address stays marked, so the school can try again.
var errGuardianEmailBlockLiftFailed = errors.New("the email block could not be lifted")

// GuardianEmailDelivery is Delivery as the guardian routes need it (#5934).
type GuardianEmailDelivery struct {
	Outcomes     deliveryRuntime.EmailOutcomes
	Suppressions deliveryRuntime.EmailSuppressions
}

// guardianEmailAudit appends the audit row of a change to a guardian's mail.
type guardianEmailAudit interface {
	Create(ctx context.Context, entry *auditModels.DataAccessLog) error
}

// guardianEmailStates reads the state of a child's guardians' addresses in the
// request's tenant transaction. The stop of a guardian is the guardian's own value.
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
			SendBlock:   string(deliveryContract.EmailSendBlockOf(marks[key], false, report.Outcome)),
			OutcomeAt:   report.OutcomeAt,
		}
	}
	return states, nil
}

// recordGuardianEmailChange writes who changed a guardian's mail. The address stays
// out: the row names the guardian.
func recordGuardianEmailChange(r *http.Request, audit guardianEmailAudit, resourceType, action string, guardianID int64) error {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	now := time.Now()
	return audit.Create(r.Context(), &auditModels.DataAccessLog{
		ActorAccountID: int64(claims.ID), ActorRole: guardianActorRole(claims.Roles),
		ResourceType: resourceType, RangeStart: now, RangeEnd: now, AccessedAt: now,
		Metadata: map[string]any{"guardian_profile_id": guardianID, "action": action},
	})
}

// liftGuardianEmailBlockFailure keeps what the school can act on and turns the
// rest into a failure worth another try. An address nothing holds is free.
func liftGuardianEmailBlockFailure(err error) error {
	switch {
	case err == nil, errors.Is(err, deliveryContract.ErrEmailSuppressionNotFound):
		return nil
	case errors.Is(err, deliveryContract.ErrEmailSuppressionNeedsReview),
		errors.Is(err, deliveryContract.ErrEmailSuppressionStaysAtProvider):
		return err
	default:
		return fmt.Errorf("%w: %w", errGuardianEmailBlockLiftFailed, err)
	}
}

// guardianEmailStopFlows binds the school's two writes on a guardian's mail. The
// stop runs in the request's transaction, the lift outside of it because it waits
// for the provider; its audit row then opens a transaction of its own.
func guardianEmailStopFlows(delivery GuardianEmailDelivery, guardians peoplePorts.GuardianService, audit guardianEmailAudit, runtime peopleCompose.LegacyRepositoryRuntime, logger *slog.Logger) (
	func(*http.Request, int64, bool) error, func(*http.Request, int64, string) error,
) {
	setStopped := func(r *http.Request, guardianID int64, stopped bool) error {
		if err := guardians.GuardianProfiles().SetEmailsStopped(r.Context(), guardianID, stopped); err != nil {
			return err
		}
		action := "resumed"
		if stopped {
			action = "stopped"
		}
		return recordGuardianEmailChange(r, audit, auditModels.ResourceTypeGuardianEmailStop, action, guardianID)
	}
	lift := func(r *http.Request, guardianID int64, address string) error {
		tenantID, err := tenant.TenantFromContext(r.Context())
		if err != nil {
			return err
		}
		if err := liftGuardianEmailBlockFailure(delivery.Suppressions.LiftEmailSuppression(r.Context(), tenantID.Int64(), address)); err != nil {
			return err
		}
		logger.Info("guardian email block lifted", slog.Int64("guardian_profile_id", guardianID))
		return runtime.InTenantTx(r.Context(), func(txCtx context.Context) error {
			return recordGuardianEmailChange(r.WithContext(txCtx), audit, auditModels.ResourceTypeGuardianEmailLift, "lifted", guardianID)
		})
	}
	return setStopped, lift
}
