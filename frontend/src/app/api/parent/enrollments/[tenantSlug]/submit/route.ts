import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface SubmitResult {
  request_id: string;
  status_url: string;
}

/**
 * Proxy POST /api/parent/enrollments/{tenantSlug}/submit →
 * backend /parent/enrollments/{tenantSlug}/submit. The backend stamps
 * guardian_account_id from the parent JWT and skips captcha — the
 * route wrapper handles auth + 401 retry. Body is forwarded as-is.
 */
export const POST = proxyPost<SubmitResult>(
  (params) =>
    `/parent/enrollments/${requirePathSegmentParam(params, "tenantSlug")}/submit`,
);
