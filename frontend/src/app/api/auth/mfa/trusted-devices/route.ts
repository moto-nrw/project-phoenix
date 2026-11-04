import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withTenantAuth } from "~/server/auth/tenant-route";
import type { ServerAuthRequest } from "~/server/auth/route-handler";

async function GETHandler(request: ServerAuthRequest) {
  return forwardJsonPost(request, "/auth/mfa/trusted-devices", {
    sessionToken: request.auth?.user?.token ?? null,
    method: "GET",
    hasBody: false,
  });
}

export const GET = withTenantAuth(GETHandler);
