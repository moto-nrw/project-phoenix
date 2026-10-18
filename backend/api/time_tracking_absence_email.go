package api

import (
	"context"
	"log/slog"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	timeTrackingHTTP "github.com/moto-nrw/project-phoenix/modules/workforce/http/timetracking"

	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"

	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// The absence request mails of the time-tracking services go out through
// Delivery, address the staff members of the People Directory and link to
// the school's subdomain from Organization & Tenancy.

type absenceEmailDispatcher struct {
	dispatcher interface {
		Dispatch(context.Context, email.DeliveryRequest)
	}
	from     email.Email
	identity email.ReplyToResolver
	logger   *slog.Logger
}

func (d absenceEmailDispatcher) Dispatch(ctx context.Context, value workforceCompose.AbsenceEmailMessage) {
	d.dispatcher.Dispatch(ctx, d.request(ctx, value))
}

func (d absenceEmailDispatcher) request(ctx context.Context, value workforceCompose.AbsenceEmailMessage) email.DeliveryRequest {
	message := email.Message{
		From: d.from, To: email.NewEmail(value.To.Name, value.To.Address),
		Subject: value.Subject, Template: value.Template, Content: value.TemplateData(),
	}
	if value.TenantID > 0 {
		identity := email.ResolveReplyToIdentity(ctx, d.identity, value.TenantID, d.logger)
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
	school, found, err := lookupSchool(ctx, d.schools, id)
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
	rows, err := d.source.ListStaffWithPermission(ctx, timeTrackingHTTP.AbsenceApproverPermission)
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
