package api

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/services"
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
	content := map[string]any{
		"FirstName":        value.Content.FirstName,
		"LastName":         value.Content.LastName,
		"AbsenceTypeLabel": value.Content.AbsenceTypeLabel,
		"DateRange":        value.Content.DateRange,
		"LinkURL":          value.Content.LinkURL,
		"LogoURL":          value.Content.LogoURL,
	}
	if value.Template == "absence-request-received.html" {
		content["RequesterName"] = value.Content.RequesterName
		content["Note"] = value.Content.Note
		content["PreviousQuestion"] = value.Content.PreviousQuestion
	} else {
		content["DecisionNote"] = value.Content.DecisionNote
	}
	message := email.Message{
		From: d.from, To: email.NewEmail(value.To.Name, value.To.Address),
		Subject: value.Subject, Template: value.Template, Content: content,
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
	school, found, err := services.FindSchool(ctx, d.schools, id)
	return school.Subdomain, found, err
}

type absenceEmailStaffRecords interface {
	GetStaffContactInfo(context.Context, int64) (*users.StaffWithRoleInfo, error)
	ListStaffWithPermission(context.Context, string) ([]*users.StaffWithRoleInfo, error)
}

type absenceEmailStaffDirectory struct{ source absenceEmailStaffRecords }

func (d absenceEmailStaffDirectory) GetStaffContactInfo(ctx context.Context, id int64) (*workforceCompose.AbsenceEmailContact, error) {
	row, err := d.source.GetStaffContactInfo(ctx, id)
	return absenceEmailContact(row), err
}

func (d absenceEmailStaffDirectory) ListAbsenceApprovers(ctx context.Context) ([]*workforceCompose.AbsenceEmailContact, error) {
	rows, err := d.source.ListStaffWithPermission(ctx, "vacation:approve")
	if rows == nil {
		return nil, err
	}
	contacts := make([]*workforceCompose.AbsenceEmailContact, len(rows))
	for i, row := range rows {
		contacts[i] = absenceEmailContact(row)
	}
	return contacts, err
}

func absenceEmailContact(row *users.StaffWithRoleInfo) *workforceCompose.AbsenceEmailContact {
	if row == nil {
		return nil
	}
	return &workforceCompose.AbsenceEmailContact{StaffID: row.StaffID, FirstName: row.FirstName, LastName: row.LastName, Email: row.Email}
}
