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
  role?: {
    id: number;
    name: string;
  };
  creator?: {
    id: number;
    email: string;
  };
}

interface CreateInvitationPayload {
  email: string;
  role_id: number;
  first_name?: string;
  last_name?: string;
}

export const GET = createGetHandler<BackendInvitation[]>(async (_request: NextRequest, token: string) => {
  return await apiGet<BackendInvitation[]>("/auth/invitations", token);
});

export const POST = createPostHandler<BackendInvitation, CreateInvitationPayload>(
  async (_request: NextRequest, body: CreateInvitationPayload, token: string) => {
    const payload: CreateInvitationPayload = {
      email: body.email,
      role_id: body.role_id,
      first_name: body.first_name,
      last_name: body.last_name,
    };

    return await apiPost<BackendInvitation, CreateInvitationPayload>("/auth/invitations", token, payload);
  }
);
