package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// PasswordResetDeliveryRecords record a reset mail's outcome in the store,
// under the module's unit of work and in the module's error vocabulary. The
// composition hands them to the root's mail delivery when it builds the
// reset flows, so the delivery needs no reference back to the module (#4080).
type PasswordResetDeliveryRecords struct {
	store   ports.PasswordResetStore
	attach  func(context.Context) context.Context
	runtime ports.Runtime
}

var _ identityaccess.PasswordResetDeliveryRecorder = PasswordResetDeliveryRecords{}

// NewPasswordResetDeliveryRecords records through store under attach.
func NewPasswordResetDeliveryRecords(store ports.PasswordResetStore, attach func(context.Context) context.Context, runtime ports.Runtime) PasswordResetDeliveryRecords {
	return PasswordResetDeliveryRecords{store: store, attach: attach, runtime: runtime}
}

func (r PasswordResetDeliveryRecords) RecordPasswordResetDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error {
	return passwordResetError(recordPasswordResetDelivery(r.attach(ctx), r.runtime, r.store, id, domain.TokenDelivery(delivery)))
}

func recordPasswordResetDelivery(ctx context.Context, runtime ports.Runtime, store ports.PasswordResetStore, id int64, delivery domain.TokenDelivery) error {
	return globalIdentityRead(ctx, runtime, func(txCtx context.Context) error {
		_, err := store.RecordPasswordResetDelivery(txCtx, id, delivery.Bounded())
		return err
	})
}

// SchoolInvitationDeliveryRecords record an invitation mail's outcome in the
// store, like PasswordResetDeliveryRecords for the reset mail.
type SchoolInvitationDeliveryRecords struct {
	store   ports.SchoolInvitationStore
	attach  func(context.Context) context.Context
	runtime ports.Runtime
}

var _ identityaccess.SchoolInvitationDeliveryRecorder = SchoolInvitationDeliveryRecords{}

// NewSchoolInvitationDeliveryRecords records through store under attach.
func NewSchoolInvitationDeliveryRecords(store ports.SchoolInvitationStore, attach func(context.Context) context.Context, runtime ports.Runtime) SchoolInvitationDeliveryRecords {
	return SchoolInvitationDeliveryRecords{store: store, attach: attach, runtime: runtime}
}

func (r SchoolInvitationDeliveryRecords) RecordSchoolInvitationDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error {
	return invitationError(recordSchoolInvitationDelivery(r.attach(ctx), r.runtime, r.store, id, domain.TokenDelivery(delivery)))
}

func recordSchoolInvitationDelivery(ctx context.Context, runtime ports.Runtime, store ports.SchoolInvitationStore, id int64, delivery domain.TokenDelivery) error {
	return globalIdentityRead(ctx, runtime, func(txCtx context.Context) error {
		_, err := store.RecordSchoolInvitationDelivery(txCtx, id, delivery.Bounded())
		return err
	})
}
