import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

async function DELETEHandler(
  request: NextRequest,
  context: { params: Promise<{ id: string }> },
) {
  const { id } = await context.params;
  return forwardJsonPost(request, `/operator/auth/mfa/trusted-devices/${id}`, {
    method: "DELETE",
    hasBody: false,
  });
}

export const DELETE = withPrivateNoStore(DELETEHandler);
