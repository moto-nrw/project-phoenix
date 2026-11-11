package owners

import (
	"context"
	"log/slog"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// HolidayClosureMail is the Träger outbox of the closing day notices with
// the transport outcome and the protected repetition of their mails
// (ADR 0196).
type HolidayClosureMail struct {
	Delivery deliveryRuntime.Capability
	deliveryRuntime.TraegerMails
}

// EnqueueTraegerMail enqueues the notice as a mail of the Träger in the
// caller's Träger transaction.
func (m HolidayClosureMail) EnqueueTraegerMail(ctx context.Context, mail enrollment.TraegerMail) error {
	_, err := m.Delivery.EnqueueEmail(ctx, deliveryContract.EmailIntent{
		OrganizationID: mail.OrganizationID, Template: mail.Kind, Recipient: deliveryContract.EmailRecipient{Address: mail.Recipient},
		Payload: mail.Payload, IdempotencyKey: mail.IdempotencyKey,
		Related: deliveryContract.RelatedEntity{Type: mail.RelatedType, ID: mail.RelatedID},
	})
	return err
}

// TraegerMailStates names the transport outcome of each mail: accepted is
// "sent", whatever the mailbox did later; a dead letter whose worker
// stopped during the provider call is "unknown"; any other dead letter or a
// mail the lists held back is "failed".
func (m HolidayClosureMail) TraegerMailStates(ctx context.Context, organizationID int64, keys []string) ([]enrollment.TraegerMailState, error) {
	statuses, err := m.TraegerEmailStatusesByKey(ctx, organizationID, keys)
	if err != nil {
		return nil, err
	}
	states := make([]enrollment.TraegerMailState, 0, len(statuses))
	for _, status := range statuses {
		if status.IdempotencyKey == nil {
			continue
		}
		outcome := enrollment.TraegerMailQueued
		switch status.State {
		case deliveryContract.StateSent:
			outcome = enrollment.TraegerMailSent
		case deliveryContract.StateDeadLetter:
			outcome = enrollment.TraegerMailFailed
			if status.LastError != nil && *status.LastError == deliveryContract.DispatchOutcomeUnknown {
				outcome = enrollment.TraegerMailUnknown
			}
		case deliveryContract.StateCancelled:
			outcome = enrollment.TraegerMailFailed
		}
		states = append(states, enrollment.TraegerMailState{IdempotencyKey: *status.IdempotencyKey, Outcome: outcome})
	}
	return states, nil
}

// NewHolidayClosures binds Enrollment's closing day notices (#6434, ADR 0196)
// to the host schools' calendar, the worker's read channel, the Träger
// directory, the outbox and Security Runtime's key primitives.
func NewHolidayClosures(holidayCalendar enrollment.HolidaySchoolCalendar, holidays HolidayClosureHolidays, traeger enrollment.HolidayTraegerDirectory,
	mail HolidayClosureMail,
) enrollment.HolidayClosures {
	return enrollmentCompose.NewHolidayClosures(holidayCalendar, NewHolidayClosureCalendar(holidays), traeger, mail,
		securityruntime.FillRandom, securityruntime.KeyedFingerprint, nil)
}

// HolidayOfficeOrganizations is what the Träger directory of a holiday mail
// reads of Organisation & Tenancy.
type HolidayOfficeOrganizations interface {
	ListSchoolsByOrganization(ctx context.Context, organizationID int64) ([]organizationtenancy.School, error)
}

// NewHolidayOfficeTraeger names the Träger and its host schools in a mail
// the Träger-Büro or its worker triggers. It reads both in the caller's
// Träger transaction: the schools like the host-school check of the holiday
// calendar, the name through Organisation & Tenancy's read of it, because
// that transaction reads no row of the organization itself.
func NewHolidayOfficeTraeger(organizations HolidayOfficeOrganizations) enrollment.HolidayTraegerDirectory {
	return holidayOfficeTraeger{organizations: organizations, names: organizationCompose.NewTraegerMasterData()}
}

type holidayOfficeTraeger struct {
	organizations HolidayOfficeOrganizations
	names         organizationtenancy.TraegerNames
}

func (d holidayOfficeTraeger) HolidayTraeger(ctx context.Context, organizationID int64) (enrollment.HolidayTraeger, error) {
	name, err := d.names.TraegerName(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayTraeger{}, err
	}
	schools, err := d.organizations.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayTraeger{}, err
	}
	traeger := enrollment.HolidayTraeger{Name: name, Schools: []enrollment.HolidaySchool{}}
	for _, school := range schools {
		if !school.IsDeleted() {
			traeger.Schools = append(traeger.Schools, enrollment.HolidaySchool{ID: school.ID, Name: school.Name})
		}
	}
	return traeger, nil
}

// HolidayClosureNotificationsJob binds the Worker's processing of confirmed
// closing day notices (ADR 0196). The Worker's context carries no unit of
// work; the job attaches unit, as the HTTP middleware does for a request.
func HolidayClosureNotificationsJob(unit tenant.UnitOfWork, holidayCalendar enrollment.HolidaySchoolCalendar, holidays HolidayClosureHolidays,
	organizations HolidayOfficeOrganizations, mail HolidayClosureMail, logger *slog.Logger,
) schedulerPorts.HolidayClosureNotifications {
	notices := NewHolidayClosureNotifications(NewHolidayClosures(holidayCalendar, holidays, NewHolidayOfficeTraeger(organizations), mail), organizations, mail,
		logger.With("workflow", "holiday-closure-notification"))
	watchdog := schoolCalendarCompose.NewHolidayClosureEvents()
	return schedulerPorts.HolidayClosureNotifications{
		ProcessDue: func(ctx context.Context) error { return notices.ProcessDue(tenant.WithUnitOfWork(ctx, unit)) },
		Overdue: func(ctx context.Context, overdueFor time.Duration) (count int, err error) {
			err = tenant.WithinAdmin(tenant.WithUnitOfWork(ctx, unit), func(ctx context.Context) error {
				count, err = watchdog.OverdueHolidayClosureEvents(ctx, overdueFor)
				return err
			})
			return count, err
		},
	}
}
