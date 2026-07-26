import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";

export async function DELETE(
  request: NextRequest,
  context: { params: Promise<{ id: string }> },
) {
  const { id } = await context.params;
  return forwardJsonPost(request, `/auth/passkeys/${id}`, {
    method: "DELETE",
    hasBody: false,
  });
}
