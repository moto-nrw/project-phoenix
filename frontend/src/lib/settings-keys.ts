// Keys whose value rides on /auth/tenant/resolve. The settings PUT/DELETE
// proxies (tenant-side `/api/settings/values/[key]` and operator-side
// `/api/operator/provisioning/schools/[id]/settings/values/[key]`) consult
// this set after a successful write and purge the cached layout fetch via
// `revalidateTag('tenant-${slug}')` + `revalidatePath('/${slug}', 'layout')`.
// The settings page also calls `router.refresh()` so the RSC tree pulls the
// fresh layout data.
export const TENANT_RESOLVE_AFFECTING_KEYS: ReadonlySet<string> = new Set([
  "operations.student_photos_enabled",
]);
