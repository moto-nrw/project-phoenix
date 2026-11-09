package test

import "maps"

// transactionFrameBundleSites are the boundaries ADR 0161 adds: one
// administrative read unit replaces consecutive same-scope units. The public
// calendar feed also keeps the parent feed's school transaction under a new
// key, unchanged, because the feed now renders it without a second token
// lookup (ADR 0161, Nachtrag). Step 4 adds the one-way admin-to-tenant
// handoff: runtime, composition, caller and the parents portal's runtime
// port, which only forwards (ADR 0161, Nachtrag zu Schritt 4). Step 3 keeps
// the attachment audience's administrative unit under a new key with its
// reason unchanged, and step 5 hands the attachment read off from that
// audience to the school (#5894). A further site needs its own decision.
var transactionFrameBundleSites = map[string]string{
	// Admin, Schritt 1 (#5881)
	"modules/schoolcalendar/portal/internal/application/feed.go:*service.calendarFeedOwnerByToken:s.cfg.Runtime.WithinAdmin#1": "ADR 0161: the public calendar feed discovers the token owner in one administrative read unit, parent token first and the staff membership token with its account only on a miss; it writes nothing and ends before the school's tenant projection.",
	// Kommando, Schritt 1 (#5881): verlagert aus
	// feed.go:*service.ParentCalendarFeedByToken:s.cfg.Runtime.WithinTenant#1
	"modules/schoolcalendar/portal/internal/application/feed.go:*service.parentCalendarFeed:s.cfg.Runtime.WithinTenant#1": "Atomic owner command: ParentCalendarFeedByToken coordinates appointment visibility and school calendar projections through application ports. Its prerequisite reads/locks, mutations and commit hooks form one command and must roll back together. It joins a supplied workflow/root unit; SQL adapters below it require that unit and cannot open one.",
	// Admin, Schritt 2 (#5795)
	"modules/identityaccess/internal/application/operator.go:*Service.VerifyOperatorSession:s.tx.RunAdmin#1": "ADR 0161: the active-operator check reads the operator and the latest session of its family in one administrative read unit with unchanged order and guards; it writes nothing and the route keeps its own administrative unit.",
	// Admin, Schritt 4 (#5862): Laufzeit, Komposition, Aufrufer, Port-Bindung
	"tenant/runtime.go:WithinAdminThenTenant:runtime.withinAdminThenTenant#1":                                        "ADR 0161: the one-way handoff discovers as phoenix_admin and continues in the same transaction as phoenix_tenant for the discovered school only; no administrative context survives the switch.",
	"modules/transactionruntime/compose/tenant_runtime.go:NewTenantRuntime:unitOfWork.WithinAdminThenTenant#1":       "ADR 0161: binds the one-way admin-to-tenant handoff of the unit of work.",
	"workflows/parentportal/internal/care/child.go:*Service.WithPermittedChild:s.Runtime.WithinAdminThenTenant#1":    "ADR 0161: a parent child read discovers active schools and the relationship-level parent_portal permission before any tenant statement, then reads the child's school under RLS in the same transaction; it writes nothing.",
	"roots/owners/parent_portal_runtime.go:ParentPortalRuntime.WithinAdminThenTenant:tenant.WithinAdminThenTenant#1": "ADR 0161: binds the one-way admin-to-tenant handoff to the parents portal workflow's runtime port; it only forwards and opens no unit of its own.",
	// Admin, Schritt 5 (#5894): Übergabe der Anhangliste
	"workflows/parentportal/internal/messaging/parent_announcement_attachments.go:*Service.WithinGuardianAnnouncementTenant:s.Runtime.WithinAdminThenTenant#1": "ADR 0161: a guardian's attachment read discovers the live announcement and the account's audience before any tenant statement, then reads the school's switch and the attachment rows under that school's RLS in the same transaction; it writes nothing, and the post-read audience check keeps its own unit.",
	// Kommando, Schritt 4 (#5862)
	"modules/transactionruntime/internal/adapters/postgres/tenant_runtime.go:*PostgresUnitOfWork.WithinAdminThenTenant:r.runInTx#1": "ADR 0161: implements the one-way admin-to-tenant handoff; role and school are transaction-local.",
	// Admin, Schritt 3 (#5894): verlagert aus
	// parent_announcement_attachments.go:*Service.GuardianAnnouncementTenant:s.Runtime.WithinAdmin#1
	"workflows/parentportal/internal/messaging/parent_announcement_attachments.go:*Service.guardianAnnouncementAudience:s.Runtime.WithinAdmin#1": "Authenticated parent messaging/declaration workflow reads or stamps account-addressed items across schools. Explicit account, child/guardian relationship and announcement audience predicates remain authoritative, including per-child permissions.",
}

// withoutReviewedTransactionFrameBundles removes exactly the reviewed
// boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedTransactionFrameBundles(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range transactionFrameBundleSites {
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
