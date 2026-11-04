package owners

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// Nachweise (#4808): the root binds Workforce's qualification ports to the
// People Directory's staff rows, Delivery's notifications and mail outbox and
// the school of Organisation & Tenancy.

// QualificationStaffRows are the People Directory's staff reads the
// Nachweise need.
type QualificationStaffRows interface {
	ListAllWithPerson(ctx context.Context) ([]*peopleValues.Staff, error)
	ListStaffWithPermission(ctx context.Context, permissionName string) ([]*peopleValues.StaffWithRoleInfo, error)
	GetStaffContactInfo(ctx context.Context, staffID int64) (*peopleValues.StaffWithRoleInfo, error)
}

// NewQualificationStaff binds Workforce's staff port to the staff rows.
func NewQualificationStaff(rows QualificationStaffRows) workforce.QualificationStaff {
	if rows == nil {
		panic("qualification staff: staff rows are required")
	}
	return qualificationStaff{rows: rows}
}

type qualificationStaff struct{ rows QualificationStaffRows }

// ListStaffMembers returns the live staff with a person, external caregivers
// included: § 72a SGB VIII asks for their certificate of conduct as well.
func (s qualificationStaff) ListStaffMembers(ctx context.Context) ([]workforce.QualificationStaffMember, error) {
	rows, err := s.rows.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	members := make([]workforce.QualificationStaffMember, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.Person == nil {
			continue
		}
		member := workforce.QualificationStaffMember{StaffID: row.ID, FirstName: row.Person.FirstName, LastName: row.Person.LastName}
		if row.Person.AccountID != nil {
			member.AccountID = *row.Person.AccountID
		}
		members = append(members, member)
	}
	return members, nil
}

func (s qualificationStaff) StaffContact(ctx context.Context, staffID int64) (workforce.QualificationStaffMember, error) {
	row, err := s.rows.GetStaffContactInfo(ctx, staffID)
	if err != nil || row == nil {
		return workforce.QualificationStaffMember{StaffID: staffID}, err
	}
	return qualificationContact(row), nil
}

func (s qualificationStaff) StammdatenManagers(ctx context.Context) ([]workforce.QualificationStaffMember, error) {
	rows, err := s.rows.ListStaffWithPermission(ctx, permissions.StaffStammdaten)
	if err != nil {
		return nil, err
	}
	members := make([]workforce.QualificationStaffMember, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			members = append(members, qualificationContact(row))
		}
	}
	return members, nil
}

func qualificationContact(row *peopleValues.StaffWithRoleInfo) workforce.QualificationStaffMember {
	return workforce.QualificationStaffMember{StaffID: row.StaffID, FirstName: row.FirstName, LastName: row.LastName, AccountID: row.AccountID, Email: row.Email}
}

// BuildQualificationReminders composes the Nachweis reminders of the Worker.
func BuildQualificationReminders(
	db transactionruntime.Database, notifications DeliveryNotifications, staff QualificationStaffRows,
	outbox OutboxEnqueuer, mailFrom email.Email, mailIdentity email.ReplyToResolver,
	schools organizationtenancy.Query, frontendURL string, logger *slog.Logger,
) (workforce.QualificationReminders, error) {
	if notifications.Notifier == nil || notifications.Preferences == nil || staff == nil || outbox.Runtime == nil || schools == nil {
		return nil, errors.New("qualification reminders: notifications, staff, delivery and schools are required")
	}
	logger = logger.With("service", "qualification_reminders")
	return workforceCompose.NewQualifications(db.Pool(), NewQualificationStaff(staff), workforce.QualificationReminderPorts{
		Staff:       NewQualificationStaff(staff),
		Notify:      qualificationNotifier(notifications, logger),
		Mail:        qualificationMailOutbox{outbox: outbox, from: mailFrom, identity: mailIdentity, logger: logger},
		Schools:     payrollSchools{schools: schools},
		FrontendURL: frontendURL, Logger: logger,
	}, nil)
}

// qualificationNotifier sends a reminder to the recipients who did not
// switch it off (the Leitung's type is on by default) or, for the person's
// own reminder, who switched it on. A school that turned notifications off
// gets the mail only; the notification window holds a notice back for a
// later run.
func qualificationNotifier(notifications DeliveryNotifications, logger *slog.Logger) workforce.QualificationNotifier {
	return func(ctx context.Context, notice workforce.QualificationNotice) (bool, error) {
		kind := deliveryContract.NotificationStaffQualificationExpiry
		filter := notifications.Preferences.FilterNotOptedOut
		if notice.Own {
			kind = deliveryContract.NotificationStaffQualificationOwnExpiry
			filter = notifications.Preferences.FilterOptedIn
		}
		recipients, err := filter(ctx, kind, notice.RecipientAccountIDs)
		if err != nil {
			return false, fmt.Errorf("filter qualification reminder recipients: %w", err)
		}
		if len(recipients) == 0 {
			return true, nil
		}
		err = notifications.Notifier.Notify(ctx, deliveryCompose.NotificationEvent{
			Type: kind, IdempotencyKey: notice.IdempotencyKey, RelatedType: "staff", RelatedID: notice.StaffID,
			Title: notice.Title, Body: notice.Body, DeepLink: notice.DeepLink,
			Audience: deliveryCompose.NotificationAudience{
				TenantID: notice.TenantID, Scope: deliveryCompose.NotificationAudienceScope("staff"), StaffAccountIDs: recipients,
			},
		})
		switch {
		case errors.Is(err, deliveryCompose.ErrNotificationsDisabled):
			return true, nil
		case errors.Is(err, deliveryCompose.ErrOutsideNotificationWindow):
			logger.Debug("qualification reminder outside the notification window",
				"qualification_id", notice.QualificationID,
				"stage", notice.Stage,
			)
			return false, nil
		}
		return err == nil, err
	}
}

// qualificationMailOutbox carries the reminder mails through Delivery's
// durable outbox, with the school's reply-to address.
type qualificationMailOutbox struct {
	outbox   OutboxEnqueuer
	from     email.Email
	identity email.ReplyToResolver
	logger   *slog.Logger
}

func (o qualificationMailOutbox) Enqueue(ctx context.Context, message workforce.QualificationMailMessage) error {
	content := message.Content
	mail := email.Message{
		From: o.from, To: email.NewEmail(message.ToName, message.ToAddress),
		Subject: message.Subject, Template: message.Template,
		Content: map[string]any{
			"SchoolName": content.SchoolName, "FirstName": content.FirstName, "StaffName": content.StaffName,
			"QualificationName": content.QualificationName, "ExpiresOn": content.ExpiresOn,
			"ExpiryDay": content.ExpiryDay, "Lapsed": content.Lapsed, "Own": content.Own, "LinkURL": content.LinkURL,
		},
	}
	identity := email.ResolveReplyToIdentity(ctx, o.identity, message.TenantID, o.logger)
	mail.ReplyTo = email.NewEmail(identity.Name, identity.Address)
	_, err := o.outbox.EnqueueMailIntent(ctx, DurableMail{
		IdempotencyKey: message.IdempotencyKey, TenantID: message.TenantID, Message: mail,
		Type: message.Type, ReferenceID: message.ReferenceID,
	})
	return err
}

// WithQualificationReminders completes the planning with the Nachweis
// reminders once it was built without error (#4808).
func (w *WorkforcePlanning) WithQualificationReminders(
	built error, db transactionruntime.Database, env RootEnvironment, notifications DeliveryNotifications, staff QualificationStaffRows,
	outbox OutboxEnqueuer, mailFrom email.Email, mailIdentity email.ReplyToResolver, schools organizationtenancy.Query,
) error {
	if built != nil {
		return built
	}
	var err error
	w.QualificationReminders, err = BuildQualificationReminders(db, notifications, staff, outbox, mailFrom, mailIdentity, schools, env.Links.FrontendURL, env.Logger)
	return err
}

// qualificationRemindersJob binds the Worker's job; a composition without
// the reminders leaves the job out.
func qualificationRemindersJob(reminders workforce.QualificationReminders) schedulerPorts.StaffQualificationReminders {
	if reminders == nil {
		return nil
	}
	return reminders.RunDue
}
