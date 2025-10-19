import type { NextRequest } from "next/server";
import { apiDelete } from "~/lib/api-helpers";
import { createDeleteHandler } from "~/lib/route-wrapper";

export const DELETE = createDeleteHandler<null>(
  async (_request: NextRequest, token: string, params: Record<string, unknown>) => {
    const rawId = params.id ?? params.invitationId;
    if (typeof rawId !== "string" && typeof rawId !== "number") {
      throw new Error("Missing invitation id");
    }

    const invitationId = String(rawId);
    await apiDelete(`/auth/invitations/${invitationId}`, token);
    return null;
  }
);
