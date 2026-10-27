import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

async function PUTHandler(
  request: NextRequest,
  context: { params: Promise<{ accountId: string }> },
) {
  const { accountId } = await context.params;
  return forwardJsonPost(
    request,
    `/auth/accounts/${encodeURIComponent(accountId)}/mfa/override`,
    { method: "PUT", hasBody: true },
  );
}

export const PUT = withPrivateNoStore(PUTHandler);
