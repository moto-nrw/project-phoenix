import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import { proxyGet } from "~/lib/route-proxy.server";

interface BackendInvitation {
  id: number;
  email: string;
  role_id: string;
  token: string;
  expires_at: string;
  created_by: number;
  first_name?: string | null;
  last_name?: string | null;
  position?: string | null;
  role?: {
    id: string;
    name: string;
  };
  creator?: {
    id: number;
    email: string;
  };
}

interface BackendResponse<T> {
  status: string;
  data: T;
  message?: string;
}

interface IncomingCreateInvitationPayload {
  email: string;
  role_id?: string | number;
  roleId?: string | number;
  first_name?: string;
  firstName?: string;
  last_name?: string;
  lastName?: string;
  position?: string;
}

interface BackendCreateInvitationPayload {
  email: string;
  role_id: string;
  first_name?: string;
  last_name?: string;
  position?: string;
}

function normalizeRoleId(
  roleId: string | number | undefined,
): string | undefined {
  if (typeof roleId === "string") {
    return /^[1-9]\d*$/.test(roleId) ? roleId : undefined;
  }
  // A parsed JSON number above MAX_SAFE_INTEGER has already lost its original
  // digits. Reject it rather than forwarding a rounded, valid role ID.
  if (
    typeof roleId === "number" &&
    Number.isSafeInteger(roleId) &&
    roleId > 0
  ) {
    return String(roleId);
  }
  return undefined;
}

export const GET = proxyGet<BackendInvitation[]>("/auth/invitations");

export const POST = createPostHandler<
  BackendInvitation,
  IncomingCreateInvitationPayload
>(
  async (
    _request: NextRequest,
    body: IncomingCreateInvitationPayload,
    token: string,
  ) => {
    const roleId = normalizeRoleId(body.role_id ?? body.roleId);

    if (roleId === undefined) {
      throw new TypeError("Invalid invitation payload: role id missing");
    }

    const payload: BackendCreateInvitationPayload = {
      email: body.email,
      role_id: roleId,
      first_name: body.first_name ?? body.firstName,
      last_name: body.last_name ?? body.lastName,
      position: body.position,
    };

    const response = await apiPost<
      BackendResponse<BackendInvitation>,
      BackendCreateInvitationPayload
    >("/auth/invitations", token, payload);

    return response.data;
  },
);
