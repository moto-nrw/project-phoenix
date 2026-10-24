package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrivilegedProvisioningRejectsAuditFailure(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"rotate", "transfer", "delete-device", "delete-school", "delete-organization", "delete-person"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seedDeviceSchools(h)
			h.devices.add(terminal(200, 10, "AUDIT-DEVICE"))
			h.addOrg(300, "Audit Organization", "audit-org", false)
			h.audit.err = errBoom
			var err error
			switch action {
			case "rotate":
				_, err = h.svc.SetDeviceAPIKey(context.Background(), 200, nil, testOperatorID, operatorIP)
			case "transfer":
				_, err = h.svc.TransferDevice(context.Background(), 200, 20, testOperatorID, operatorIP)
			case "delete-device":
				err = h.svc.DeleteDevice(context.Background(), 200, testOperatorID, operatorIP)
			case "delete-school":
				err = h.svc.SoftDeleteSchool(context.Background(), 10, testOperatorID, operatorIP)
			case "delete-organization":
				err = h.svc.SoftDeleteOrganization(context.Background(), 300, testOperatorID, operatorIP)
			case "delete-person":
				h.people.persons[400] = domain.Person{ID: 400, TenantID: 10}
				err = h.svc.SoftDeletePerson(context.Background(), 400, testOperatorID, operatorIP)
			}
			require.ErrorIs(t, err, errBoom)
			assert.ErrorIs(t, h.tx.lastErr, errBoom, "audit error must escape the administrative transaction")
		})
	}
}
