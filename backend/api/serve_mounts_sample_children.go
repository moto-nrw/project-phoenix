package api

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	importapi "github.com/moto-nrw/project-phoenix/modules/dataimport/http"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// newSampleChildrenRoute serves POST /operator/organizations/{id}/sample-children
// (#4658): Data Import fills the test schools of a test Träger, which
// Organisation & Tenancy names, through the server's student import. Without
// the student import the route is not mounted.
func newSampleChildrenRoute(provisioning organizationtenancy.TestTraegerProvisioning, students importModels.RowImporter[importModels.StudentImportRow]) http.HandlerFunc {
	if provisioning == nil || students == nil {
		return nil
	}
	workflow := importModels.NewSampleChildren(testTraegerSchools{provisioning: provisioning}, students, withinSchool, calendar.TodayDate)
	return importapi.SampleChildrenHandler(workflow, importapi.OperatorRuntime{
		ParseID: common.ParseIDParam, OperatorID: operatorIDFromSession, ClientIP: common.ParseClientIP,
		Success: common.Respond, Failure: renderDataImportFailure,
	})
}

// testTraegerSchools binds Data Import's test schools to Organisation &
// Tenancy and translates its refusals into the Data Import contract.
type testTraegerSchools struct {
	provisioning organizationtenancy.TestTraegerProvisioning
}

func (s testTraegerSchools) RecordSampleChildrenRequest(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) ([]int64, error) {
	ids, err := s.provisioning.RecordSampleChildrenRequest(ctx, organizationID, operatorID, clientIP)
	var notFound *organizationtenancy.OrganizationNotFoundError
	var auditUnavailable *organizationtenancy.OperatorAuditUnavailableError
	switch {
	case err == nil:
		return ids, nil
	case errors.Is(err, organizationtenancy.ErrNotTestTraeger):
		return nil, importModels.ErrNotTestTraeger
	case errors.Is(err, organizationtenancy.ErrOrganizationInactive):
		return nil, importModels.ErrTestTraegerInactive
	case errors.As(err, &notFound):
		return nil, importModels.ErrTestTraegerNotFound
	case errors.As(err, &auditUnavailable):
		return nil, errors.Join(importModels.ErrSampleAuditUnavailable, err)
	default:
		return nil, err
	}
}
