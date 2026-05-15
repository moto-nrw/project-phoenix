import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy";

export async function GET(
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

export async function DELETE(
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
