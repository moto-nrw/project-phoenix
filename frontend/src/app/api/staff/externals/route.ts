// app/api/staff/externals/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import type { ExternalAccess } from "~/lib/staff-api";
import {
  mapBackendStaff,
  type BackendStaffResponse,
} from "~/lib/staff-route-mapping";

interface ExternalStaffCreateRequest {
  first_name: string;
  last_name: string;
  access: ExternalAccess;
  email?: string;
  organization?: string;
}

/**
 * Handler for POST /api/staff/externals
 * Records an external caregiver (#3823) with the chosen access (ADR 0088):
 * `none` without account, `view` and `supervise` with an email invitation.
 * The backend validates names, access and address; this hop only forwards
 * them.
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
      access: body.access,
    },
  );
  return mapBackendStaff(response.data);
});
