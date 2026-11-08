package test

import "maps"

// transactionFrameBundleSites are the boundaries ADR 0161 adds: one
// administrative read unit replaces consecutive same-scope units. The public
// calendar feed also keeps the parent feed's school transaction under a new
// key, unchanged, because the feed now renders it without a second token
// lookup (ADR 0161, Nachtrag), and the attachment audience keeps its
// administrative unit under a new key, its callback unchanged (ADR 0161,
// Nachtrag zu Schritt 3). A further site needs its own decision.
var transactionFrameBundleSites = map[string]string{
	// Admin, Schritt 1 (#5881)
	"modules/schoolcalendar/portal/internal/application/feed.go:*service.calendarFeedOwnerByToken:s.cfg.Runtime.WithinAdmin#1": "ADR 0161: the public calendar feed discovers the token owner in one administrative read unit, parent token first and the staff membership token with its account only on a miss; it writes nothing and ends before the school's tenant projection.",
	// Kommando, Schritt 1 (#5881): verlagert aus
	// feed.go:*service.ParentCalendarFeedByToken:s.cfg.Runtime.WithinTenant#1
	"modules/schoolcalendar/portal/internal/application/feed.go:*service.parentCalendarFeed:s.cfg.Runtime.WithinTenant#1": "Atomic owner command: ParentCalendarFeedByToken coordinates appointment visibility and school calendar projections through application ports. Its prerequisite reads/locks, mutations and commit hooks form one command and must roll back together. It joins a supplied workflow/root unit; SQL adapters below it require that unit and cannot open one.",
	// Admin, Schritt 2 (#5795)
	"modules/identityaccess/internal/application/operator.go:*Service.VerifyOperatorSession:s.tx.RunAdmin#1": "ADR 0161: the active-operator check reads the operator and the latest session of its family in one administrative read unit with unchanged order and guards; it writes nothing and the route keeps its own administrative unit.",
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
