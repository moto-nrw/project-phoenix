import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface BackendStaffPreviewCandidate {
  // int64 on the backend, serialized as a string (see staff-preview-api.ts).
  account_id: string;
  first_name: string;
  last_name: string;
  email: string;
  roles: string[];
}

/**
 * GET /api/auth/staff-preview/candidates
 * Lists the staff members an admin can preview (#2893).
 */
export const GET = createGetHandler(async (_request, token) => {
  const response = await apiGet<{ data: BackendStaffPreviewCandidate[] }>(
    "/auth/staff-preview/candidates",
    token,
  );
  return response.data;
});
