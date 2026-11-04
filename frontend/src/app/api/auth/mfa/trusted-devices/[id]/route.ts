import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withTenantAuth } from "~/server/auth/tenant-route";
import type { ServerAuthRequest } from "~/server/auth/route-handler";

async function DELETEHandler(
  request: ServerAuthRequest,
  context: { params: Promise<{ id: string }> },
) {
  const { id } = await context.params;
  return forwardJsonPost(request, `/auth/mfa/trusted-devices/${id}`, {
    sessionToken: request.auth?.user?.token ?? null,
    method: "DELETE",
    hasBody: false,
  });
}

export const DELETE = withTenantAuth(DELETEHandler);
