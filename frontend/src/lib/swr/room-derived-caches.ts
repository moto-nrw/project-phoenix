/**
 * Single source of truth for SWR cache keys that hold room-stamped data.
 *
 * **Why this file exists**: many pages cache student/visit lists with
 * `current_room_color` (or other room-derived attributes like the room name)
 * baked into each row. When an admin saves a Room — colour, name, anything
 * the badge renderer reads — those caches stay stale until SWR refetches.
 * The Database Rooms page invalidates them via `useTenantMutateMatching`,
 * but it needs to know which cache substrings count as "room-derived".
 *
 * Hardcoding that list inside `database/rooms/page.tsx` worked but rotted on
 * contact: every new SWR consumer that consumes room-stamped data needs the
 * page-author to also remember to register their key here. Centralising
 * makes the coupling visible — anyone touching this list is forced past the
 * doc comment that explains the invariant.
 *
 * **What belongs here**: cache-key substrings (matching the
 * `useTenantMutateMatching` convention of trailing-dash prefixes) for caches
 * that contain ANY room-derived value:
 *   - `current_room_color` (Issue #1324)
 *   - the resolved room name in `current_location` strings
 *   - any future room-attribute that finds its way into a student/visit row
 *
 * **What does NOT belong**: caches that fetch Room data directly
 * (`/api/rooms` lists). Invalidate those through the rooms resource family. This
 * list is for *consumers* of room data, not the room data itself.
 *
 * If you add a new SWR key that fits the criteria above, add its substring
 * here AND add a unit test verifying the badge updates after a room save.
 * The forward-compat hazard documented on `useTenantMutateMatching` makes
 * silent regressions easy.
 */

/**
 * Substrings of cache keys that hold room-derived student/visit data.
 *
 * Each entry uses the convention from `useTenantMutateMatching`: include the
 * trailing dash before the first dynamic segment so accidental future
 * collisions are harder.
 *
 * Keep this list in sync with the actual SWR keys used in pages. Currently:
 *   - `active-supervision-dashboard-${refreshKey}` — active-supervisions
 *     aggregate (carries the selected session's visits since #2096)
 *   - `ogs-students-${groupId}` — OGS-Groups aggregated live view (#2056;
 *     `ogs-students-auto` on a cold start before a group is selected)
 *   - `search-students-g${groupFilter}-${term}-…` — Students Search list
 *     (the leading group scope segment is the #2097 SSE scoping anchor, see
 *     `lib/swr/search-students-key.ts`)
 */
export const ROOM_DERIVED_CACHE_KEY_FRAGMENTS: readonly string[] = [
  "active-supervision-dashboard-",
  "ogs-students-",
  "search-students-",
] as const;
