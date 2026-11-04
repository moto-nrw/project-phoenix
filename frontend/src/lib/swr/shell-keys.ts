/**
 * Keys the tenant shell reads on every route, including the public ones
 * (#5007). Kept apart from portal-keys.ts, which bundles as a whole.
 */
export const announcementKeys = {
  unread: () => "user-announcements-unread",
} as const;

// Shared by the home page, its now strip and the rooms page; the SSE hook
// refreshes it on attendance events.
export const dashboardKeys = {
  analytics: () => "dashboard-analytics",
  supervision: (accountId: string | undefined, refresh = 0) =>
    `active-supervision-dashboard-${refresh}-${accountId}`,
} as const;

// „Aktuelle Aufsicht" without groups:read (#4573): the caller's own planned
// blocks and running sessions. The SSE consumer invalidates the family on
// supervision, activity and timetable events (#5017).
export const ownSupervisionKeys = {
  family: "active-supervisions-own-",
  day: () => "active-supervisions-own-day",
  plannedNow: () => "active-supervisions-own-planned-now",
  sessions: () => "active-supervisions-own-sessions",
} as const;

// Family prefixes of the shared resource factories, here so the
// always-mounted SSE invalidator carries no key factory.
export const SWR_FAMILIES = {
  rooms: "rooms-",
  groups: "groups-",
  staff: "staff-directory-",
  me: "me-",
} as const;
