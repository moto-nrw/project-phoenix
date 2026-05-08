// Setting keys whose value travels through `/auth/tenant/resolve` and is
// served to every page via `[tenant]/layout.tsx → TenantContext`.
//
// Toggling one of these has two cache-busting consequences that diverged
// between the settings page and the PUT/DELETE proxy if their copies of
// this set drifted:
//
//   1. The originating tab needs `router.refresh()` so the cached
//      `[tenant]/layout.tsx` RSC fetch (tag: `tenant-${slug}`) re-runs and
//      TenantContext picks up the new value.
//   2. The PUT/DELETE proxy needs to call `revalidateTag("tenant-${slug}",
//      { expire: 0 })` so the next refresh sees fresh data instead of the
//      5-minute-stale resolve response.
//
// Both call sites read from this constant — keeping the list in one place
// prevents a new tenant-resolve-affecting key being added in only one of
// them.
export const TENANT_RESOLVE_AFFECTING_KEYS: ReadonlySet<string> = new Set([
  "operations.student_photos_enabled",
]);
