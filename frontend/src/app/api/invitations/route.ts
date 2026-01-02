import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

interface BackendInvitation {
  id: number;
  email: string;
  role_id: number;
  token: string;
  expires_at: string;
  created_by: number;
  first_name?: string | null;
  last_name?: string | null;
  position?: string | null;
  role?: {
    id: number;
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
  role_id?: number;
  roleId?: number;
  first_name?: string;
  firstName?: string;
  last_name?: string;
  lastName?: string;
  position?: string;
}

interface BackendCreateInvitationPayload {
  email: string;
  role_id: number;
  first_name?: string;
  last_name?: string;
  position?: string;
}

export const GET = createGetHandler<BackendInvitation[]>(
  async (_request: NextRequest, token: string) => {
    const response = await apiGet<BackendResponse<BackendInvitation[]>>(
      "/auth/invitations",
      token,
    );
    return response.data;
  },
);

export const POST = createPostHandler<
  BackendInvitation,
  IncomingCreateInvitationPayload
>(
  async (
    _request: NextRequest,
    body: IncomingCreateInvitationPayload,
    token: string,
  ) => {
    const roleId = body.role_id ?? body.roleId;

    if (typeof roleId !== "number") {
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
