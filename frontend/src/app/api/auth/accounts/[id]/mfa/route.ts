import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy";

export async function DELETE(
  request: NextRequest,
  context: { params: Promise<{ id: string }> },
) {
  const { id } = await context.params;
  return forwardJsonPost(
    request,
    `/auth/accounts/${encodeURIComponent(id)}/mfa`,
    { method: "DELETE", hasBody: true },
  );
}
