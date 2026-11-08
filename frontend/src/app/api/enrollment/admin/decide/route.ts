import { enrollmentChildPath } from "~/lib/enrollment-admin-child.server";
import { tenant } from "~/lib/route-wrapper.server";
import { parseRequestBody } from "~/lib/route-wrapper-utils.server";

// Flattened proxy. The backend handler lives at
// /api/enrollment/admin/requests/{id}/children/{childId}/decide, but
// Turbopack dev (Next.js 16) loses that route's registration after
// its periodic filesystem-cache compaction, returning 404 until the
// dev server restarts. Moving the proxy to a single non-dynamic path
// and reading the IDs from the body sidesteps the bug.

interface DecideBody {
  request_id?: string | number;
  child_id?: string | number;
  status?: string;
  reason?: string;
}

export const POST = tenant.route(async (request) => {
  const body = await parseRequestBody<DecideBody>(request);
  return tenant.forwardToBackend(request, enrollmentChildPath(body, "decide"), {
    method: "POST",
    json: { status: body.status, reason: body.reason ?? "" },
  });
});
