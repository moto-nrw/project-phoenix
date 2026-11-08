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
	{http.MethodPost, "/traeger/holiday-periods/{id}/registrations/{registrationId}/freeze"}:                     notCaptured,
	{http.MethodPost, "/traeger/holiday-periods/{id}/registrations/{registrationId}/reopen"}:                     notCaptured,
	{http.MethodPut, "/traeger/holiday-periods/{id}/registrations/{registrationId}/children/{childId}/bookings"}: notCaptured,
	{http.MethodPut, "/traeger/holiday-periods/{id}/registrations/{registrationId}/details"}:                     notCaptured,
	{http.MethodPost, "/traeger/school-sessions/"}:                                                               notCaptured,
	{http.MethodDelete, "/traeger/settings/values/{key}"}:                                                        notCaptured,
	{http.MethodPut, "/traeger/settings/values/{key}"}:                                                           notCaptured,
	{http.MethodPut, "/traeger/master-data/contact"}:                                                             notCaptured,
	{http.MethodPut, "/traeger/master-data/withdrawal-contact"}:                                                  notCaptured,
	{http.MethodPut, "/traeger/master-data/logo"}:                                                                notCaptured,
	{http.MethodDelete, "/traeger/master-data/logo"}:                                                             notCaptured,
	{http.MethodPost, "/traeger/master-data/bank-accounts"}:                                                      notCaptured,
	{http.MethodPut, "/traeger/master-data/bank-accounts/{id}"}:                                                  notCaptured,
	{http.MethodDelete, "/traeger/master-data/bank-accounts/{id}"}:                                               notCaptured,
	// The document templates (#4677) belong to the Träger: no school to
	// count them for. The preview stores nothing.
	{http.MethodPost, "/traeger/master-data/document-templates/{kind}/versions"}: notCaptured,
	{http.MethodPost, "/traeger/master-data/document-templates/{kind}/preview"}:  notCaptured,
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
	// The debtor number of a confirmed registration (#5621) belongs to the
	// Träger as well.
	{http.MethodPut, "/traeger/holiday-contributions/{periodId}/registrations/{registrationId}/debtor-number"}: notCaptured,
	// The cost bearers of a Träger and the cost takeover (#5627) belong to
	// the Träger too, and so does the proof the Träger-Büro uploads.
	{http.MethodPost, "/traeger/cost-bearers/"}:                                              notCaptured,
	{http.MethodPut, "/traeger/cost-bearers/{id}"}:                                           notCaptured,
	{http.MethodDelete, "/traeger/cost-bearers/{id}"}:                                        notCaptured,
	{http.MethodPost, "/traeger/holiday-periods/{id}/registrations/{registrationId}/proofs"}: notCaptured,
	// The siblings of separate holiday registrations (#5628) belong to the
	// Träger too.
	{http.MethodPost, "/traeger/holiday-contributions/{periodId}/sibling-groups"}:                                  notCaptured,
	{http.MethodDelete, "/traeger/holiday-contributions/{periodId}/sibling-groups/registrations/{registrationId}"}: notCaptured,
	// The cancellation of a holiday registration moved to the
	// Elternabrechnung, which computes the sibling group again (#5662).
	{http.MethodPost, "/traeger/holiday-contributions/{periodId}/registrations/{registrationId}/cancel"}: notCaptured,
	// The billing run of a holiday period (#4668, ADR 0119) belongs to the
	// Träger: no school to count it for.
	{http.MethodPost, "/traeger/billing-runs/"}:                 notCaptured,
	{http.MethodPost, "/traeger/billing-runs/{id}/corrections"}: notCaptured,
	{http.MethodPost, "/traeger/billing-runs/{id}/release"}:     notCaptured,
	// The corrections a reissued invoice carries over (#5663).
	{http.MethodPut, "/traeger/billing-runs/{id}/carried-corrections/{positionID}"}:    notCaptured,
	{http.MethodDelete, "/traeger/billing-runs/{id}/carried-corrections/{positionID}"}: notCaptured,
	{http.MethodPut, "/traeger/number-ranges/{service}"}:                               notCaptured,
	{http.MethodPut, "/traeger/billing-document-kinds/{service}"}:                      notCaptured,
	// The confirmation letters of a released run (#4669, ADR 0121) belong
	// to the Träger as well.
	{http.MethodPost, "/traeger/billing-runs/{id}/letters"}:         notCaptured,
	{http.MethodPost, "/traeger/billing-runs/{id}/letters/preview"}: notCaptured,
	// The mail of the letters (#4670, ADR 0123) belongs to the Träger too.
	{http.MethodPost, "/traeger/billing-runs/{id}/mailings"}:                        notCaptured,
	{http.MethodPost, "/traeger/billing-runs/{id}/documents/{documentID}/mailings"}: notCaptured,
	// Storno and Gutschrift of an issued invoice (#4690) belong to the
	// Träger as well.
	{http.MethodPost, "/traeger/billing-runs/{id}/documents/{documentID}/cancellation"}: notCaptured,
	{http.MethodPost, "/traeger/billing-runs/{id}/documents/{documentID}/credits"}:      notCaptured,
	// The monthly contribution of the care contract (#4684): the price list
	// and the monthly runs of a Träger, no school to count it for.
	{http.MethodPut, "/traeger/care-tariffs/"}:                               notCaptured,
	{http.MethodDelete, "/traeger/care-tariffs/{schoolID}/{validFrom}"}:      notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/"}:                         notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/release"}:             notCaptured,
	{http.MethodPut, "/traeger/care-meal-tariffs/"}:                          notCaptured,
	{http.MethodDelete, "/traeger/care-meal-tariffs/{schoolID}/{validFrom}"}: notCaptured,
	{http.MethodPost, "/traeger/care-rules/"}:                                notCaptured,
	{http.MethodPut, "/traeger/care-rules/{id}"}:                             notCaptured,
	{http.MethodDelete, "/traeger/care-rules/{id}"}:                          notCaptured,
	{http.MethodPost, "/traeger/care-evidences/"}:                            notCaptured,
	{http.MethodPut, "/traeger/care-evidences/{id}"}:                         notCaptured,
	{http.MethodDelete, "/traeger/care-evidences/{id}"}:                      notCaptured,
	{http.MethodPost, "/traeger/care-cost-shares/"}:                          notCaptured,
	{http.MethodPut, "/traeger/care-cost-shares/{id}"}:                       notCaptured,
	{http.MethodDelete, "/traeger/care-cost-shares/{id}"}:                    notCaptured,
	// Payment terms, family deviations and due dates of a run (#4689)
	// belong to the Träger, no school to count them for.
	{http.MethodPut, "/traeger/billing-terms/{service}"}:            notCaptured,
	{http.MethodPut, "/traeger/family-billing/"}:                    notCaptured,
	{http.MethodDelete, "/traeger/family-billing/{id}"}:             notCaptured,
	{http.MethodPut, "/traeger/billing-run-due-dates/care/{id}"}:    notCaptured,
	{http.MethodPut, "/traeger/billing-run-due-dates/holiday/{id}"}: notCaptured,
	// The letters, mails and Storno of a monthly run (#4684, ADR 0165).
	{http.MethodPost, "/traeger/care-billing-runs/{id}/letters"}:                           notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/letters/preview"}:                   notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/letters/mail"}:                      notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/invoices/{invoiceID}/mail"}:         notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/invoices/{invoiceID}/cancellation"}: notCaptured,
	// The review of a monthly run before its release (#5916).
	{http.MethodPut, "/traeger/care-billing-runs/{id}/invoice-date"}:               notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/exclusions"}:                notCaptured,
	{http.MethodDelete, "/traeger/care-billing-runs/{id}/exclusions/{contractID}"}: notCaptured,
	// The children of a family (#5918): the Abrechnung confirms or removes.
	{http.MethodPut, "/traeger/family-children/"}:                notCaptured,
	{http.MethodDelete, "/traeger/family-children/{contractId}"}: notCaptured,
	// The care contracts and their templates (#4678, ADR 0153) belong to
	// the Träger, not to a school.
	{http.MethodPost, "/traeger/contracts/"}:       notCaptured,
	{http.MethodPost, "/traeger/contracts/send"}:   notCaptured,
	{http.MethodDelete, "/traeger/contracts/{id}"}: notCaptured,
	// The paper contract (#4679) is recorded by the office, no core action.
	{http.MethodPost, "/traeger/contracts/{id}/paper-signature"}:            notCaptured,
	{http.MethodPut, "/traeger/contract-templates/{schoolId}/{offeringId}"}: notCaptured,
	// Withdrawal and termination (#4682): the Träger-Büro records a
	// declaration it received and sets the end of an extraordinary
	// termination it checked; the parents' own one in the parents portal.
	{http.MethodPost, "/traeger/contract-notices/{id}"}:                                      notCaptured,
	{http.MethodPut, "/traeger/contract-notices/{id}/ends-on"}:                               notCaptured,
	{http.MethodPost, "/traeger/contract-notices/{id}/reason-rejection"}:                     notCaptured,
	{http.MethodPost, "/parent/me/documents/{organizationId}/contracts/{contractId}/notice"}: notCaptured,
	// The parents state the children of their family on a contract (#5918).
	{http.MethodPut, "/parent/me/documents/{organizationId}/contracts/{contractId}/family-children"}: notCaptured,
}

func init() {
	for key, action := range traegerCoreActions {
		if _, ok := coreActions[key]; ok {
			panic(fmt.Sprintf("analytics: route %s %s classified twice", key.Method, key.Pattern))
		}
		coreActions[key] = action
	}
}
