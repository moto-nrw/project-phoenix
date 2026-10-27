import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

async function GETHandler(
  request: NextRequest,
  context: { params: Promise<{ accountId: string }> },
) {
  const { accountId } = await context.params;
  return forwardJsonPost(
    request,
    `/auth/accounts/${encodeURIComponent(accountId)}/mfa`,
    { method: "GET", hasBody: false },
  );
}

async function DELETEHandler(
  request: NextRequest,
  context: { params: Promise<{ accountId: string }> },
) {
  const { accountId } = await context.params;
  return forwardJsonPost(
    request,
    `/auth/accounts/${encodeURIComponent(accountId)}/mfa`,
    { method: "DELETE", hasBody: true },
  );
}

export const GET = withPrivateNoStore(GETHandler);
export const DELETE = withPrivateNoStore(DELETEHandler);
