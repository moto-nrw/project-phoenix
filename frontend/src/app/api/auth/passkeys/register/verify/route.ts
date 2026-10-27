import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withTenantAuth } from "~/server/auth/tenant-route";
import type { ServerAuthRequest } from "~/server/auth/route-handler";

async function POSTHandler(request: ServerAuthRequest) {
  return forwardJsonPost(request, "/auth/passkeys/register/verify", {
    sessionToken: request.auth?.user?.token ?? null,
  });
}

export const POST = withTenantAuth(POSTHandler);
