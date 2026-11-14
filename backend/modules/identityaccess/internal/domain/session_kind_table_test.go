package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Only a Träger session, an OGS session the Träger opened and a parent
// session at a Träger name a Träger, each with the school its kind demands;
// of these only the OGS session is kept at a school.
func TestSessionRowsNameATraegerOnlyInTheirKind(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		scope           string
		tenantID, orgID int64
		holds, atSchool bool
		kind            SessionKind
	}{
		{PortalOrg.Name, 0, 7, true, false, SessionKind(PortalOrg.Name)},
		{PortalOrg.Name, 3, 7, false, false, SessionKind(PortalOrg.Name)},
		{PortalOrg.Name, 0, 0, false, false, SessionKind(PortalOrg.Name)},
		{PortalTenant.Name, 3, 0, true, true, SessionKind(PortalTenant.Name)},
		{PortalTenant.Name, 3, 7, true, true, SessionKindTraegerSchool},
		{PortalTenant.Name, 0, 7, false, true, SessionKindTraegerSchool},
		{PortalParent.Name, 3, 0, true, true, SessionKind(PortalParent.Name)},
		{PortalParent.Name, 0, 7, true, false, SessionKindTraegerParent},
		{PortalParent.Name, 3, 7, false, false, SessionKindTraegerParent},
		{PortalSchool.Name, 3, 0, true, true, SessionKind(PortalSchool.Name)},
		{PortalSchool.Name, 3, 7, false, true, SessionKind(PortalSchool.Name)},
		{PortalCaterer.Name, 3, 0, true, true, SessionKind(PortalCaterer.Name)},
		{PortalCaterer.Name, 3, 7, false, true, SessionKind(PortalCaterer.Name)},
		{PortalScopeUnknown, 3, 0, true, true, SessionKind(PortalScopeUnknown)},
		{PortalScopeUnknown, 3, 7, false, true, SessionKind(PortalScopeUnknown)},
	} {
		session := AccountSession{PortalScope: tc.scope, TenantID: tc.tenantID, OrganizationID: tc.orgID}
		assert.Equal(t, tc.kind, session.SessionKind(), "%+v", tc)
		assert.Equal(t, tc.holds, session.traegerShapeHolds(), "%+v", tc)
		assert.Equal(t, tc.atSchool, session.KeptAtSchool(), "%+v", tc)
	}
}
