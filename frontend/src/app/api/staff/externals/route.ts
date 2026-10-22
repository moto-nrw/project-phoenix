// app/api/staff/externals/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import {
  mapBackendStaff,
  type BackendStaffResponse,
} from "~/lib/staff-route-mapping";

interface ExternalStaffCreateRequest {
  first_name: string;
  last_name: string;
  email: string;
  organization?: string;
}

/**
 * Handler for POST /api/staff/externals
 * Records an external caregiver (#3823) and invites them by email with the
 * school role „Extern“ (#4573). The backend validates names and address;
 * this hop only forwards them.
 */
export const POST = createPostHandler<
  ReturnType<typeof mapBackendStaff>,
  ExternalStaffCreateRequest
>(async (_request: NextRequest, body, token) => {
  // apiPost hands back the backend envelope as is; the staff record sits in
  // its `data` member.
  const response = await apiPost<{ data: BackendStaffResponse }>(
    "/api/staff/externals",
    token,
    {
      first_name: body.first_name,
      last_name: body.last_name,
      email: body.email,
      organization: body.organization,
    },
  );
  return mapBackendStaff(response.data);
});
