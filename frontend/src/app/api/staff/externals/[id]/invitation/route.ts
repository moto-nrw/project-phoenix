// app/api/staff/externals/[id]/invitation/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { bffError } from "~/lib/api-response-error.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import {
  mapBackendStaff,
  type BackendStaffResponse,
} from "~/lib/staff-route-mapping";

interface ExternalInvitationRequest {
  email: string;
}

const DECIMAL_ID = /^\d+$/;

/**
 * Handler for POST /api/staff/externals/[id]/invitation
 * Invites an external caregiver who has no account yet (#4573). The id is the
 * staff id as a decimal string; it is forwarded as is, never as a number.
 */
export const POST = createPostHandler<
  ReturnType<typeof mapBackendStaff>,
  ExternalInvitationRequest
>(async (_request: NextRequest, body, token, params) => {
  const id = params.id;
  if (typeof id !== "string" || !DECIMAL_ID.test(id)) {
    throw bffError(400, "Invalid staff id");
  }
  const response = await apiPost<{ data: BackendStaffResponse }>(
    `/api/staff/externals/${id}/invitation`,
    token,
    { email: body.email },
  );
  return mapBackendStaff(response.data);
});
