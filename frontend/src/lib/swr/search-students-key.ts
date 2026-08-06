/**
 * The Kindersuche list cache key and its group scope (#2097).
 *
 * **Why the key carries a scope segment.** `search-students-*` used to hang on
 * every tenant-wide `dashboard_counts_changed`, so a single check-in anywhere
 * in the school made every open Kindersuche tab refetch the whole list. #2057
 * gave the check-in/checkout/count events an educational `group_ids` scope; to
 * use it here the invalidation matcher must be able to read the page's group
 * filter back out of its own cache key.
 *
 * Parsing it positionally is not possible: the key's first dynamic segment is
 * the free-text search term, which may itself contain dashes ("Anna-Lena").
 * So the scope is encoded in a fixed, unambiguous segment right after the
 * prefix — `search-students-g7-…` for the group filter "7",
 * `search-students-gall-…` when no group is selected — and matched with the
 * same whole-segment `keyTargetsId` boundary rule the `ogs-students-{gid}`
 * keys use ("g7" must never match "g70").
 *
 * A view with no group filter still refetches on every check-in: it renders
 * every child of the school with a live location badge, so a check-in in ANY
 * group changes a visible row. Only the group-filtered views can be narrowed.
 */

/** Prefix shared by every Kindersuche list cache key. */
export const SEARCH_STUDENTS_KEY_PREFIX = "search-students-";

/**
 * Prefix up to (excluding) the group id, for whole-segment id matching:
 * `keyTargetsId(key, groupId, [SEARCH_STUDENTS_GROUP_KEY_PREFIX])`.
 */
export const SEARCH_STUDENTS_GROUP_KEY_PREFIX = `${SEARCH_STUDENTS_KEY_PREFIX}g`;

/** The scope segment of an unfiltered ("Alle Gruppen") Kindersuche view. */
export const SEARCH_STUDENTS_ALL_GROUPS_KEY = `${SEARCH_STUDENTS_GROUP_KEY_PREFIX}all-`;

const MAX_INT64 = 9_223_372_036_854_775_807n;

/**
 * Mirrors the backend's effective `group_id` filter: only positive signed
 * 64-bit integers filter the list. Canonicalizing here keeps `007` scoped to
 * group 7, while zero, negative, overflowing and non-numeric values use the
 * unfiltered scope.
 */
export function normalizeSearchStudentsGroupId(groupId: string): string {
  if (!/^\+?\d+$/.test(groupId)) return "";

  const parsed = BigInt(groupId);
  return parsed > 0n && parsed <= MAX_INT64 ? parsed.toString() : "";
}

/**
 * The scope segment for a group filter value ("" = Alle Gruppen).
 *
 * Group ids are numeric, so `g{id}` can never collide with `gall`.
 */
export function searchStudentsGroupScope(groupId: string): string {
  const normalizedGroupId = normalizeSearchStudentsGroupId(groupId);
  return `g${normalizedGroupId === "" ? "all" : normalizedGroupId}`;
}
