import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withTenantAuth } from "~/server/auth/tenant-route";
import type { ServerAuthRequest } from "~/server/auth/route-handler";

async function GETHandler(
  request: ServerAuthRequest,
  context: { params: Promise<{ accountId: string }> },
) {
  const { accountId } = await context.params;
  return forwardJsonPost(
    request,
    `/auth/accounts/${encodeURIComponent(accountId)}/mfa`,
    {
      sessionToken: request.auth?.user?.token ?? null,
      method: "GET",
      hasBody: false,
    },
  );
}

async function DELETEHandler(
  request: ServerAuthRequest,
  context: { params: Promise<{ accountId: string }> },
) {
  const { accountId } = await context.params;
  return forwardJsonPost(
    request,
    `/auth/accounts/${encodeURIComponent(accountId)}/mfa`,
    {
      sessionToken: request.auth?.user?.token ?? null,
      method: "DELETE",
      hasBody: true,
    },
  );
}

export const GET = withTenantAuth(GETHandler);
export const DELETE = withTenantAuth(DELETEHandler);
