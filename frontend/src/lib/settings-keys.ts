// Keys whose value rides on /auth/tenant/resolve. Settings page calls
// router.refresh() and the PUT/DELETE proxy calls revalidateTag(`tenant-${slug}`)
// — both read this set, keep them in sync.
export const TENANT_RESOLVE_AFFECTING_KEYS: ReadonlySet<string> = new Set([
  "operations.student_photos_enabled",
]);
