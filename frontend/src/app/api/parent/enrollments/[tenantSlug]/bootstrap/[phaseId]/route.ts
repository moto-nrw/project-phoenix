import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import type { PublicEnrollmentBootstrap } from "~/lib/enrollment-submission-api";

/**
 * Proxy GET /api/parent/enrollments/{tenantSlug}/bootstrap/{phaseId} →
 * backend /parent/enrollments/{tenantSlug}/bootstrap/{phaseId}. Forwards the
 * parent NextAuth session token (and any late_invite query param) so a
 * logged-in guardian can load audience-restricted (linked_parents) phases,
 * which the anonymous public bootstrap endpoint refuses (#1663). tenantSlug
 * and phaseId come straight from the URL — the backend resolves the tenant
 * under WithAdminTx and runs the authenticated enrollee phase gate.
 */
export const GET = proxyGet<PublicEnrollmentBootstrap>(
  (params) =>
    `/parent/enrollments/${requirePathSegmentParam(
      params,
      "tenantSlug",
    )}/bootstrap/${requirePathSegmentParam(params, "phaseId")}`,
);
