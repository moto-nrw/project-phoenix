/**
 * Whether the school requires a guardian to give a reason when sending a
 * request (backend setting `operations.parent_request_reason_policy`,
 * exposed per child as `ChildFeatures.reason_required`).
 *
 * A missing flag means the strictest reading: keep the reason mandatory, so an
 * old backend or a failed features fetch can never let a request through that
 * the server would then reject with `reason_required`.
 */
export function requiresGuardianReason(
  features?: Readonly<{ reason_required?: boolean }>,
): boolean {
  return features?.reason_required !== false;
}
