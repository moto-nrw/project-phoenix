package owners

import (
	"context"
	"log/slog"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"

	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"

	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

type AbsenceEmailDispatcher struct {
	Durable  DurableMailOutbox
	From     email.Email
	Identity email.ReplyToResolver
	Logger   *slog.Logger
}

func (d AbsenceEmailDispatcher) Enqueue(ctx context.Context, value workforceCompose.AbsenceEmailMessage) error {
	return enqueueDurableMail(ctx, d.Durable, DurableMail{
		TenantID: value.TenantID, Message: d.request(ctx, value).Message, Type: value.Type, ReferenceID: value.ReferenceID,
		Backoff: []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute},
	})
}

func (d AbsenceEmailDispatcher) request(ctx context.Context, value workforceCompose.AbsenceEmailMessage) email.DeliveryRequest {
	message := email.Message{
		From: d.From, To: email.NewEmail(value.To.Name, value.To.Address),
		Subject: value.Subject, Template: value.Template, Content: value.TemplateData(),
	}
	if value.TenantID > 0 {
		identity := email.ResolveReplyToIdentity(ctx, d.Identity, value.TenantID, d.Logger)
		message.ReplyTo = email.NewEmail(identity.Name, identity.Address)
	}
	return email.DeliveryRequest{
		Message:  message,
		Metadata: email.DeliveryMetadata{Type: value.Type, ReferenceID: value.ReferenceID, Recipient: value.Recipient},
	}
}

type absenceEmailSchoolDirectory struct {
	schools organizationtenancy.Query
}

func (d absenceEmailSchoolDirectory) FindSchoolSubdomain(ctx context.Context, id int64) (string, bool, error) {
	school, found, err := LookupSchool(ctx, d.schools, id)
	return school.Subdomain, found, err
}

type absenceEmailStaffRecords interface {
	GetStaffContactInfo(context.Context, int64) (*peopleValues.StaffWithRoleInfo, error)
	ListStaffWithPermission(context.Context, string) ([]*peopleValues.StaffWithRoleInfo, error)
}

type absenceEmailStaffDirectory struct{ source absenceEmailStaffRecords }

func (d absenceEmailStaffDirectory) GetStaffContactInfo(ctx context.Context, id int64) (*workforceCompose.AbsenceEmailContact, error) {
	row, err := d.source.GetStaffContactInfo(ctx, id)
	return absenceEmailContact(row), err
}

func (d absenceEmailStaffDirectory) ListAbsenceApprovers(ctx context.Context) ([]*workforceCompose.AbsenceEmailContact, error) {
	rows, err := d.source.ListStaffWithPermission(ctx, permissions.VacationApprove)
	if rows == nil {
		return nil, err
	}
	contacts := make([]*workforceCompose.AbsenceEmailContact, len(rows))
	for i, row := range rows {
		contacts[i] = absenceEmailContact(row)
	}
	return contacts, err
}

func absenceEmailContact(row *peopleValues.StaffWithRoleInfo) *workforceCompose.AbsenceEmailContact {
	if row == nil {
		return nil
	}
	return &workforceCompose.AbsenceEmailContact{StaffID: row.StaffID, FirstName: row.FirstName, LastName: row.LastName, Email: row.Email}
}
