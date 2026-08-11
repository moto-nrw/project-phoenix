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
 * `search-students-g7.9-…` when several groups are selected (#2218),
 * `search-students-gall-…` when none is — and matched by
 * {@link searchStudentsKeyTargetsGroup} with the same whole-segment boundary
 * rule the `ogs-students-{gid}` keys use ("g7" must never match "g70").
 *
 * A view with no group filter still refetches on every check-in: it renders
 * every child of the school with a live location badge, so a check-in in ANY
 * group changes a visible row. Only the group-filtered views can be narrowed.
 */

/** Prefix shared by every Kindersuche list cache key. */
export const SEARCH_STUDENTS_KEY_PREFIX = "search-students-";

/**
 * Prefix up to (excluding) the group scope, used by
 * {@link searchStudentsKeyTargetsGroup} to locate that segment in a key.
 */
const SEARCH_STUDENTS_GROUP_KEY_PREFIX = `${SEARCH_STUDENTS_KEY_PREFIX}g`;

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
 * Separator between the group ids of a multi-group scope (#2218). A dot cannot
 * appear in a numeric id and is not the key's own "-" separator, so the whole
 * scope stays one readable segment: `search-students-g7.9-…`.
 */
const GROUP_SCOPE_SEPARATOR = ".";

/**
 * The scope segment for the page's group filter (empty selection = Alle
 * Gruppen). Several groups may be selected at once (#2218); their ids share one
 * segment, so a key stays parseable by {@link searchStudentsKeyTargetsGroup}
 * regardless of how many are chosen.
 *
 * Group ids are numeric, so `g{id}` can never collide with `gall`.
 */
export function searchStudentsGroupScope(groupIds: readonly string[]): string {
  const normalized = groupIds
    .map(normalizeSearchStudentsGroupId)
    .filter((groupId) => groupId !== "");
  const unique = [...new Set(normalized)];
  return `g${unique.length === 0 ? "all" : unique.join(GROUP_SCOPE_SEPARATOR)}`;
}

/**
 * Whether a Kindersuche cache key is scoped to this educational group.
 *
 * The multi-group scope is why this cannot be the generic whole-segment
 * `keyTargetsId` check: in `search-students-g7.9-…` the id 9 is not preceded by
 * the key prefix. Missing it would leave the second selected group's tab stale
 * after a check-in there. The boundary rule `keyTargetsId` exists for still
 * holds — the ids are compared whole, so group 8 never matches a scope of 80.
 */
export function searchStudentsKeyTargetsGroup(
  key: string,
  groupId: string,
): boolean {
  for (
    let at = key.indexOf(SEARCH_STUDENTS_GROUP_KEY_PREFIX);
    at !== -1;
    at = key.indexOf(SEARCH_STUDENTS_GROUP_KEY_PREFIX, at + 1)
  ) {
    // The prefix starts the key or sits right after the tenant separator —
    // the same anchoring keyTargetsId applies.
    const before = at === 0 ? undefined : key[at - 1];
    if (before !== undefined && before !== ":") continue;

    const rest = key.slice(at + SEARCH_STUDENTS_GROUP_KEY_PREFIX.length);
    const end = rest.indexOf("-");
    const scope = end === -1 ? rest : rest.slice(0, end);
    if (scope.split(GROUP_SCOPE_SEPARATOR).includes(groupId)) return true;
  }
  return false;
}
