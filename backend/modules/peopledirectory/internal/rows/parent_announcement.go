package users

// Announcement priority levels (mirrors the chk_parent_announcements_priority
// constraint). "important" only changes presentation (a highlighted badge); it
// has no effect on targeting or delivery.
const (
	ParentAnnouncementPriorityInfo      = "info"
	ParentAnnouncementPriorityImportant = "important"
)

// Response types (mirrors the chk_parent_announcements_response_type
// constraint). "none" is a plain Mitteilung; the two choice types make the
// announcement a poll (Umfrage, #1371) that guardians answer PER CHILD.
const (
	ParentAnnouncementResponseNone         = "none"
	ParentAnnouncementResponseSingleChoice = "single_choice"
	ParentAnnouncementResponseMultiChoice  = "multi_choice"
)

// Delivery modes (mirrors the chk_parent_announcements_delivery_mode
// constraint). "standard" is the Mitteilung every pre-#2384 row is: e-mail and
// acknowledgement are independent opt-ins and the mail carries only a title and
// a portal link. "letter" is the Elternbrief: both channels are mandatory (the
// chk_parent_announcements_letter_channels CHECK enforces it in the database, not
// just in the service) and the mail carries the full body.
const (
	ParentAnnouncementDeliveryStandard = "standard"
	ParentAnnouncementDeliveryLetter   = "letter"
)

// E-mail audiences (mirrors the chk_parent_announcements_email_audience
// constraint). This is deliberately a SEPARATE axis from the portal audience:
// a letter may be a general notice everyone should receive, or it may carry
// details only guardians with portal access are meant to see (#2384).
//
//	EmailAudiencePortalOnly  — only guardians with parent_portal.access, i.e. the
//	                           same people who see the announcement in the portal
//	EmailAudienceAllContacts — additionally every guardian of a reached child that
//	                           has an address but no portal access; they receive
//	                           the mail and can never acknowledge
//
// PortalOnly is the DEFAULT on purpose: an omission must fall towards not
// disclosing the body to someone deliberately excluded from the portal.
const (
	EmailAudiencePortalOnly  = "portal_only"
	EmailAudienceAllContacts = "all_contacts"
)

// Audience target types (mirrors the chk_parent_announcement_targets_type
// constraint). An announcement's reach is the OR-union of its target rows.
const (
	AnnouncementTargetSchoolAll         = "school_all"
	AnnouncementTargetClass             = "class"
	AnnouncementTargetGroup             = "group"
	AnnouncementTargetActivityGroup     = "activity_group"
	AnnouncementTargetStudent           = "student"
	AnnouncementTargetPendingEnrollment = "pending_enrollment"
)

// System kinds mark announcements the system wrote on behalf of the team
// (mirrors chk_parent_announcements_system_kind). NULL/empty is a hand-written
// announcement. A system row is published on creation and shown in the parent
// feed regardless of the optional news feature flag.
const (
	ParentAnnouncementSystemKindCareCancellation = "care_cancellation"
)
