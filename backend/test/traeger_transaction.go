package test

import "maps"

// traegerTransactionSites are the boundaries of the Träger transaction ADR
// 0086 section 5 decides and ADR 0105 introduces: the tenant runtime's entry
// point, its PostgreSQL binding and adapter, and the one HTTP middleware that
// opens it for a Träger-Portal route. ADR 0107 adds the Träger settings,
// which open it themselves to notify the schools after their own commit, and
// ADR 0121 their branding read for the letters, which #5621 shares with the
// read of the debtor number switch. ADR 0153 adds the intake of the
// Betreuungsvertrag, which reads each school in its tenant transaction and
// then writes in the Träger transaction. ADR 0155 adds the signature of
// the contract over its one-time link and the reminders of the worker. A
// further site needs its own decision.
var traegerTransactionSites = map[string]string{
	"tenant/traeger.go:WithinTraeger:runtime.withinTraeger#1":                                                                           "ADR 0086 section 5 and ADR 0105: the tenant runtime's Träger transaction entry point refuses a school, administrative or foreign Träger unit and binds the Träger for the adapter; never selected by a missing school.",
	"modules/transactionruntime/compose/tenant_runtime.go:NewTenantRuntime:unitOfWork.WithinTraeger#1":                                  "ADR 0086 section 5 and ADR 0105: Transaction Runtime binds the PostgreSQL Träger transaction into the serving unit of work; the demo runtime gets none.",
	"modules/transactionruntime/internal/adapters/postgres/tenant_runtime.go:*PostgresUnitOfWork.WithinTraeger:r.runInTx#1":             "ADR 0086 section 5 and ADR 0105: the Träger transaction runs under phoenix_tenant with app.current_org_id and no school, joins only the same Träger's ambient unit, and never assumes the administrative role.",
	"api/common/tenant_middleware.go:TraegerTxMiddleware:tenant.WithinTraeger#1":                                                        "ADR 0105: TraegerTxMiddleware opens the Träger transaction of the session's Träger for a Träger-Portal route; a request without a Träger session fails closed before the handler.",
	"modules/settings/compose/tenant_settings_runtime.go:TenantSettingsRuntime.WithinTraeger:tenant.WithinTraeger#1":                    "ADR 0086 section 5 and ADR 0107: the Settings Platform runtime binds the shared Träger transaction of the root unit of work for the Träger settings; it never selects a school or bypasses RLS.",
	"modules/settings/internal/application/traeger_settings_service.go:*TraegerSettings.Schema:t.deps.Runtime.WithinTraeger#1":          "ADR 0086 section 5 and ADR 0107: the Träger settings read the session's Träger values in its Träger transaction; RLS on config.traeger_setting_values admits only that Träger's rows.",
	"modules/settings/internal/application/traeger_settings_service.go:*TraegerSettings.traegerSnapshot:t.deps.Runtime.WithinTraeger#1": "ADR 0112, ADR 0121 and #5621 (renamed from *TraegerSettings.TraegerTexts): the confirmation letters read the effective branding and the Elternabrechnung whether the Träger asks for the debtor number, each the session's Träger value or the registry default, in the caller's Träger transaction; RLS on config.traeger_setting_values admits only that Träger's rows.",
	"modules/settings/internal/application/traeger_settings_service.go:*TraegerSettings.change:t.deps.Runtime.WithinTraeger#1":          "ADR 0086 sections 5 and 7 and ADR 0107: a Träger value change and its change-log entry commit together in the command's own Träger transaction, so the schools hear of it only after that commit.",
	"modules/carecontract/internal/application/contracts.go:*Contracts.Contract:c.runtime.WithinTraeger#1":                              "ADR 0153, ADR 0088 section 7: one contract and its trail entry are read in the Träger transaction; the read is then logged in the tenant transaction of the contract's school; no transaction carries both.",
	"modules/carecontract/internal/application/contracts.go:*Contracts.ContractDocument:c.runtime.WithinTraeger#1":                      "ADR 0153, ADR 0088 section 7: the PDF of one contract and its trail entry are read in the Träger transaction; the read is then logged in the tenant transaction of the contract's school; no transaction carries both.",
	"modules/carecontract/internal/application/contracts.go:*Contracts.ListContracts:c.runtime.WithinTraeger#1":                         "ADR 0153, ADR 0088 section 7: the list of contracts and its trail entry are read in the Träger transaction; the read is then logged in the tenant transaction of each school shown; no transaction carries both.",
	"modules/carecontract/internal/application/intake.go:*Intake.Candidates:in.runtime.WithinTraeger#1":                                 "ADR 0153: the candidates read the keys of the Träger's contracts and its templates in the Träger transaction after each school's accepted applications were read in that school's tenant transaction; no transaction carries both and it writes nothing.",
	"modules/carecontract/internal/application/intake.go:*Intake.CreateContracts:in.runtime.WithinTraeger#1":                            "ADR 0153: the drafts of the accepted applications, their versions, parties and trail entries commit together in the Träger transaction of the session's Träger, after each school was read in its own tenant transaction; a second draft for the same key is skipped.",
	"modules/carecontract/internal/application/intake.go:*Intake.SaveTemplate:in.runtime.WithinTraeger#1":                               "ADR 0153: a contract template and its trail entry commit together in the Träger transaction of the session's Träger, after the offering was found in its school's tenant transaction.",
	"modules/carecontract/internal/application/intake.go:*Intake.Templates:in.runtime.WithinTraeger#1":                                  "ADR 0153: the templates of the Träger are read in its Träger transaction after the offerings of each school were read in that school's tenant transaction; it writes nothing.",
	"modules/carecontract/internal/application/paper.go:*Signatures.PaperDocument:s.contracts.runtime.WithinTraeger#1":                  "ADR 0155, ADR 0088 section 7: the signed paper PDF and its trail entry are read in the Träger transaction; after commit the read is logged in the contract's school through logSchools, before the file leaves. No transaction carries both.",
	"modules/carecontract/internal/application/reminders.go:*Reminders.RemindDue:r.ports.Runtime.WithinTraeger#1":                       "ADR 0155: the reminders of one Träger, their new one-time links, the outbox mails and the trail entries commit together in that Träger's transaction; one Träger's failure does not hold back the next.",
	"modules/carecontract/internal/application/signing.go:*Signatures.withinLink:s.ports.Runtime.WithinTraeger#1":                       "ADR 0155: the one-time link names its Träger; the link, the contract under its row lock, the signature with its ledger entry, the conclusion and the confirmation mail commit together in that Träger's transaction.",
}

// withoutReviewedTraegerTransactions removes exactly the reviewed Träger
// transaction boundaries, with their reviewed reasons, before the
// shrink-only comparison. It is no general escape: any other new or reworded
// site stays.
func withoutReviewedTraegerTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range traegerTransactionSites {
		if current[site] != reason {
			continue
		}
		if len(remaining) == len(current) {
			remaining = maps.Clone(current)
		}
		delete(remaining, site)
	}
	return remaining
}
