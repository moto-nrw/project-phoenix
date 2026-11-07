//go:build testhooks

package api

import (
	"github.com/go-chi/chi/v5"
	staffmessaging "github.com/moto-nrw/project-phoenix/modules/communication/http/staffmessages"
)

// NewPeopleStaffRecipientsBudgetTestRouter exposes the existing recipient
// router without making an owner suite import another owner's HTTP role.
func NewPeopleStaffRecipientsBudgetTestRouter(module StaffMessagingTestModule) chi.Router {
	return staffmessaging.NewResource(module.StaffMessaging).Router(ProtectedTenantRoutes)
}
