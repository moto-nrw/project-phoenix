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
} as const;

// Family prefixes of the shared resource factories, here so the
// always-mounted SSE invalidator carries no key factory.
export const SWR_FAMILIES = {
  rooms: "rooms-",
  groups: "groups-",
  staff: "staff-directory-",
  me: "me-",
} as const;
