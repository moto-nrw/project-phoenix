import {
  createParentPostHandler,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

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
export const POST = createParentPostHandler<SubmitResult, unknown>(
  async (_request, body, token, params) => {
    const slug = typeof params.tenantSlug === "string" ? params.tenantSlug : "";
    return parentApiPost<SubmitResult>(
      `/parent/enrollments/${encodeURIComponent(slug)}/submit`,
      token,
      body,
    );
  },
);
