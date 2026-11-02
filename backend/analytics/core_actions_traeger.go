package analytics

import (
	"fmt"
	"net/http"
)

// traegerCoreActions classifies the writing routes of the Träger-Portal
// (ADR 0084). It collects nothing until a collection decision for the Träger
// exists; a Träger session names no school, so ActorOf yields no actor either.
// init merges the table into coreActions.
var traegerCoreActions = map[RouteKey]CoreAction{
	{http.MethodPost, "/traeger/auth/login"}:                            notCaptured,
	{http.MethodPost, "/traeger/auth/mfa/enroll/confirm"}:               notCaptured,
	{http.MethodPost, "/traeger/auth/mfa/enroll/start"}:                 notCaptured,
	{http.MethodPost, "/traeger/auth/mfa/resend"}:                       notCaptured,
	{http.MethodPost, "/traeger/auth/mfa/verify"}:                       notCaptured,
	{http.MethodPost, "/traeger/auth/password-reset"}:                   notCaptured,
	{http.MethodPost, "/traeger/auth/password-reset/confirm"}:           notCaptured,
	{http.MethodPost, "/traeger/accounts/invitations"}:                  notCaptured,
	{http.MethodDelete, "/traeger/accounts/invitations/{invitationId}"}: notCaptured,
	{http.MethodPut, "/traeger/accounts/{accountId}/roles"}:             notCaptured,
	{http.MethodDelete, "/traeger/accounts/{accountId}"}:                notCaptured,
	{http.MethodPost, "/traeger/invitations/{token}/accept"}:            notCaptured,
	{http.MethodPost, "/traeger/holiday-periods/"}:                      notCaptured,
	{http.MethodPut, "/traeger/holiday-periods/{id}"}:                   notCaptured,
	{http.MethodPost, "/traeger/holiday-periods/{id}/publish"}:          notCaptured,
	// The form of a holiday period (#5198) belongs to the Träger, not to a
	// school: no school to count it for.
	{http.MethodPut, "/traeger/holiday-periods/{id}/form"}: notCaptured,
	// The review of the holiday registrations (#4663) belongs to the
	// Träger, not to a school: no school to count it for.
	{http.MethodPost, "/traeger/holiday-periods/{id}/registrations/{registrationId}/cancel"}:                     notCaptured,
	{http.MethodPost, "/traeger/holiday-periods/{id}/registrations/{registrationId}/freeze"}:                     notCaptured,
	{http.MethodPost, "/traeger/holiday-periods/{id}/registrations/{registrationId}/reopen"}:                     notCaptured,
	{http.MethodPut, "/traeger/holiday-periods/{id}/registrations/{registrationId}/children/{childId}/bookings"}: notCaptured,
	{http.MethodPost, "/traeger/school-sessions/"}:                                                               notCaptured,
	{http.MethodDelete, "/traeger/settings/values/{key}"}:                                                        notCaptured,
	{http.MethodPut, "/traeger/settings/values/{key}"}:                                                           notCaptured,
	{http.MethodPut, "/traeger/master-data/contact"}:                                                             notCaptured,
	{http.MethodPut, "/traeger/master-data/logo"}:                                                                notCaptured,
	{http.MethodDelete, "/traeger/master-data/logo"}:                                                             notCaptured,
	{http.MethodPost, "/traeger/master-data/bank-accounts"}:                                                      notCaptured,
	{http.MethodPut, "/traeger/master-data/bank-accounts/{id}"}:                                                  notCaptured,
	{http.MethodDelete, "/traeger/master-data/bank-accounts/{id}"}:                                               notCaptured,
	// The public holiday registration of a Träger (#4661) and the upload of
	// its proof of benefits (#5238) have no session and no school, so no
	// collection policy admits them yet.
	{http.MethodPost, "/traeger/holiday-registrations/{token}"}:         notCaptured,
	{http.MethodPost, "/traeger/holiday-registrations/{token}/uploads"}: notCaptured,
	// The Elternabrechnung (#4664, ADR 0115) belongs to the Träger: its
	// rules, the confirmation that freezes a contribution and the public
	// contribution of the form have no school to count them for.
	{http.MethodPost, "/traeger/contribution-rules/"}:                                                     notCaptured,
	{http.MethodPut, "/traeger/contribution-rules/{id}"}:                                                  notCaptured,
	{http.MethodDelete, "/traeger/contribution-rules/{id}"}:                                               notCaptured,
	{http.MethodPost, "/traeger/holiday-contributions/{periodId}/registrations/{registrationId}/confirm"}: notCaptured,
	{http.MethodPost, "/traeger/holiday-quotes/{token}"}:                                                  notCaptured,
	// The billing run of a holiday period (#4668, ADR 0119) belongs to the
	// Träger: no school to count it for.
	{http.MethodPost, "/traeger/billing-runs/"}:                 notCaptured,
	{http.MethodPost, "/traeger/billing-runs/{id}/corrections"}: notCaptured,
	{http.MethodPost, "/traeger/billing-runs/{id}/release"}:     notCaptured,
	{http.MethodPut, "/traeger/number-ranges/{service}"}:        notCaptured,
	// The confirmation letters of a released run (#4669, ADR 0121) belong
	// to the Träger as well.
	{http.MethodPost, "/traeger/billing-runs/{id}/letters"}:         notCaptured,
	{http.MethodPost, "/traeger/billing-runs/{id}/letters/preview"}: notCaptured,
	// The mail of the letters (#4670, ADR 0123) belongs to the Träger too.
	{http.MethodPost, "/traeger/billing-runs/{id}/mailings"}:                        notCaptured,
	{http.MethodPost, "/traeger/billing-runs/{id}/documents/{documentID}/mailings"}: notCaptured,
}

func init() {
	for key, action := range traegerCoreActions {
		if _, ok := coreActions[key]; ok {
			panic(fmt.Sprintf("analytics: route %s %s classified twice", key.Method, key.Pattern))
		}
		coreActions[key] = action
	}
}
