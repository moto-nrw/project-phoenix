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
	{http.MethodPost, "/traeger/holiday-periods/{id}/registrations/{registrationId}/freeze"}: notCaptured,
	{http.MethodPost, "/traeger/holiday-periods/{id}/registrations/{registrationId}/reopen"}: notCaptured,
	{http.MethodPut, "/traeger/holiday-periods/{id}/registrations/{registrationId}/details"}: notCaptured,
	// The office's decision on a holiday care wish (#4703).
	{http.MethodPut, "/traeger/holiday-periods/{id}/registrations/{registrationId}/children/{childId}/holiday-wish"}: notCaptured,
	// The allocation of holiday places by criteria (#4700) belongs to the
	// Träger, not to a school: no school to count it for.
	{http.MethodPut, "/traeger/holiday-periods/criteria"}:                                     notCaptured,
	{http.MethodPost, "/traeger/holiday-periods/{id}/allocation/decisions"}:                   notCaptured,
	{http.MethodPut, "/traeger/holiday-periods/{id}/registrations/{registrationId}/criteria"}: notCaptured,
	// The closing days of a Träger (#4702) belong to the Träger, not to a
	// school: no school to count them for.
	{http.MethodPost, "/traeger/closing-days/"}:       notCaptured,
	{http.MethodPut, "/traeger/closing-days/{id}"}:    notCaptured,
	{http.MethodDelete, "/traeger/closing-days/{id}"}: notCaptured,
	// The preview and the notices of closing days in booked holiday weeks
	// (#6434) belong to the Träger as well.
	{http.MethodPost, "/traeger/holiday-closures/preview"}:        notCaptured,
	{http.MethodPost, "/traeger/holiday-closures/pending/notify"}: notCaptured,
	// The Büro's click that tells the parents of a changed school closing
	// day (ADR 0196); the school's click counts the same event.
	{http.MethodPost, "/traeger/holiday-closures/pending/{id}/notify"}: event("holiday_closure_parents_notified"),
	{http.MethodPost, "/traeger/school-sessions/"}:                     notCaptured,
	{http.MethodDelete, "/traeger/settings/values/{key}"}:              notCaptured,
	{http.MethodPut, "/traeger/settings/values/{key}"}:                 notCaptured,
	{http.MethodPut, "/traeger/master-data/contact"}:                   notCaptured,
	{http.MethodPut, "/traeger/master-data/withdrawal-contact"}:        notCaptured,
	{http.MethodPut, "/traeger/master-data/logo"}:                      notCaptured,
	{http.MethodDelete, "/traeger/master-data/logo"}:                   notCaptured,
	{http.MethodPost, "/traeger/master-data/bank-accounts"}:            notCaptured,
	{http.MethodPut, "/traeger/master-data/bank-accounts/{id}"}:        notCaptured,
	{http.MethodDelete, "/traeger/master-data/bank-accounts/{id}"}:     notCaptured,
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
	// The rebooking moved to the Elternabrechnung too, and the capacity of a
	// holiday week lives beside it (#4699): both can move children up the
	// waiting list and compute their contribution again. Like the rest of
	// the Träger portal, not captured.
	{http.MethodPut, "/traeger/holiday-contributions/{periodId}/registrations/{registrationId}/children/{childId}/bookings"}: notCaptured,
	{http.MethodPut, "/traeger/holiday-contributions/{periodId}/weeks/{weekId}/capacity"}:                                    notCaptured,
	// The billing run of a holiday period (#4668, ADR 0119) belongs to the
	// Träger: no school to count it for.
	{http.MethodPost, "/traeger/billing-runs/"}:                 notCaptured,
	{http.MethodPost, "/traeger/billing-runs/{id}/corrections"}: notCaptured,
	{http.MethodPost, "/traeger/billing-runs/{id}/release"}:     notCaptured,
	{http.MethodPost, "/traeger/billing-runs/{id}/discard"}:     notCaptured,
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
	{http.MethodPut, "/traeger/care-tariffs/"}:                          notCaptured,
	{http.MethodDelete, "/traeger/care-tariffs/{schoolID}/{validFrom}"}: notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/"}:                    notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/release"}:        notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/discard"}:        notCaptured,
	// The Jahresbescheide of the monthly care (#6219): Träger, no school.
	{http.MethodPost, "/traeger/care-notice-runs/"}:             notCaptured,
	{http.MethodPost, "/traeger/care-notice-runs/{id}/release"}: notCaptured,
	{http.MethodPost, "/traeger/care-notice-runs/{id}/discard"}: notCaptured,
	// The stored originals of the Jahresbescheide and their mail (#6347):
	// Träger, no school.
	{http.MethodPost, "/traeger/care-notice-runs/{id}/letters"}:                   notCaptured,
	{http.MethodPost, "/traeger/care-notice-runs/{id}/letters/mail"}:              notCaptured,
	{http.MethodPost, "/traeger/care-notice-runs/{id}/notices/{contractID}/mail"}: notCaptured,
	{http.MethodPut, "/traeger/care-meal-tariffs/"}:                               notCaptured,
	{http.MethodDelete, "/traeger/care-meal-tariffs/{schoolID}/{validFrom}"}:      notCaptured,
	{http.MethodPost, "/traeger/care-rules/"}:                                     notCaptured,
	{http.MethodPut, "/traeger/care-rules/{id}"}:                                  notCaptured,
	{http.MethodDelete, "/traeger/care-rules/{id}"}:                               notCaptured,
	{http.MethodPost, "/traeger/care-evidences/"}:                                 notCaptured,
	{http.MethodPut, "/traeger/care-evidences/{id}"}:                              notCaptured,
	{http.MethodDelete, "/traeger/care-evidences/{id}"}:                           notCaptured,
	{http.MethodPost, "/traeger/care-cost-shares/"}:                               notCaptured,
	{http.MethodPut, "/traeger/care-cost-shares/{id}"}:                            notCaptured,
	{http.MethodDelete, "/traeger/care-cost-shares/{id}"}:                         notCaptured,
	// Payment terms, family deviations and due dates of a run (#4689)
	// belong to the Träger, no school to count them for.
	{http.MethodPut, "/traeger/billing-terms/{service}"}:            notCaptured,
	{http.MethodPut, "/traeger/family-billing/"}:                    notCaptured,
	{http.MethodPut, "/traeger/billing-families/debtor-number"}:     notCaptured,
	{http.MethodDelete, "/traeger/family-billing/{id}"}:             notCaptured,
	{http.MethodPut, "/traeger/billing-run-due-dates/care/{id}"}:    notCaptured,
	{http.MethodPut, "/traeger/billing-run-due-dates/holiday/{id}"}: notCaptured,
	// The SEPA creditor and the mandates of the families (#4695) belong to
	// the Träger, no school to count them for.
	{http.MethodPut, "/traeger/sepa-creditor/"}:                      notCaptured,
	{http.MethodPost, "/traeger/sepa-mandates/"}:                     notCaptured,
	{http.MethodPut, "/traeger/sepa-mandates/{id}"}:                  notCaptured,
	{http.MethodPost, "/traeger/sepa-mandates/{id}/revoke"}:          notCaptured,
	{http.MethodPost, "/traeger/sepa-debits/{kind}/{id}/submission"}: notCaptured,
	// The field mapping of the Sollstellung and the start of the file
	// numbers (#4693) belong to the Träger, no school to count them for.
	{http.MethodPut, "/traeger/finance-exports/settings/sollstellung"}:      notCaptured,
	{http.MethodDelete, "/traeger/finance-exports/settings/sollstellung"}:   notCaptured,
	{http.MethodPut, "/traeger/finance-exports/settings/file-number-start"}: notCaptured,
	// The revenue accounts and cost centers (#4821) belong to the Träger,
	// no school to count them for.
	{http.MethodPut, "/traeger/finance-exports/booking-accounts/revenue-accounts"}:     notCaptured,
	{http.MethodPost, "/traeger/finance-exports/booking-accounts/cost-centers"}:        notCaptured,
	{http.MethodDelete, "/traeger/finance-exports/booking-accounts/cost-centers/{id}"}: notCaptured,
	// The import of existing mandates (#6198) belongs to the Träger, not to
	// a school; the preview stores nothing.
	{http.MethodPost, "/traeger/sepa-mandates/imports"}:         notCaptured,
	{http.MethodPost, "/traeger/sepa-mandates/imports/preview"}: notCaptured,
	// The letters, mails and Storno of a monthly run (#4684, ADR 0165).
	{http.MethodPost, "/traeger/care-billing-runs/{id}/letters"}:                           notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/letters/preview"}:                   notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/letters/mail"}:                      notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/invoices/{invoiceID}/mail"}:         notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/invoices/{invoiceID}/cancellation"}: notCaptured,
	// The review of a monthly run before its release (#5916).
	{http.MethodPut, "/traeger/care-billing-runs/{id}/invoice-date"}: notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/exclusions"}:  notCaptured,
	{http.MethodPost, "/traeger/care-billing-runs/{id}/inclusions"}:  notCaptured,
	// The changes of a single invoice in the review (#6197).
	{http.MethodPost, "/traeger/care-billing-runs/{id}/amendments"}: notCaptured,
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
	// The parents give and revoke the SEPA mandate of their family (#4696);
	// reading the text before the confirmation writes nothing.
	{http.MethodPost, "/parent/me/documents/{organizationId}/contracts/{contractId}/sepa-mandate"}:         event("sepa_mandate_granted"),
	{http.MethodPost, "/parent/me/documents/{organizationId}/contracts/{contractId}/sepa-mandate/revoke"}:  event("sepa_mandate_revoked"),
	{http.MethodPost, "/parent/me/documents/{organizationId}/contracts/{contractId}/sepa-mandate/preview"}: notCaptured,
}

func init() {
	for key, action := range traegerCoreActions {
		if _, ok := coreActions[key]; ok {
			panic(fmt.Sprintf("analytics: route %s %s classified twice", key.Method, key.Pattern))
		}
		coreActions[key] = action
	}
}
