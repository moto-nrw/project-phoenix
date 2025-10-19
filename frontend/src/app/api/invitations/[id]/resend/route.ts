import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

export const POST = createPostHandler<null, Record<string, never>>(
  async (_request: NextRequest, _body: Record<string, never>, token: string, params: Record<string, unknown>) => {
    const rawId = params.id ?? params.invitationId;
    if (typeof rawId !== "string" && typeof rawId !== "number") {
      throw new Error("Missing invitation id");
    }

    const invitationId = String(rawId);
    await apiPost(`/auth/invitations/${invitationId}/resend`, token);
    return null;
  }
);
