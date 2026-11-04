import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withTenantAuth } from "~/server/auth/tenant-route";
import type { ServerAuthRequest } from "~/server/auth/route-handler";

async function PUTHandler(
  request: ServerAuthRequest,
  context: { params: Promise<{ accountId: string }> },
) {
  const { accountId } = await context.params;
  return forwardJsonPost(
    request,
    `/auth/accounts/${encodeURIComponent(accountId)}/mfa/override`,
    {
      sessionToken: request.auth?.user?.token ?? null,
      method: "PUT",
      hasBody: true,
    },
  );
}

export const PUT = withTenantAuth(PUTHandler);
