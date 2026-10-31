/**
 * Family prefixes of the shared resource factories (#5007). A module of its
 * own, so the always-mounted SSE invalidator carries no key factory.
 */
export const SWR_FAMILIES = {
  rooms: "rooms-",
  groups: "groups-",
  staff: "staff-directory-",
  me: "me-",
} as const;
