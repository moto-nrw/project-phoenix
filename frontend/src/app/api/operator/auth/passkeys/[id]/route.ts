import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withOperatorAuth } from "~/server/auth/operator-route";
import type { ServerAuthRequest } from "~/server/auth/route-handler";

async function DELETEHandler(
  request: ServerAuthRequest,
  context: { params: Promise<{ id: string }> },
) {
  const { id } = await context.params;
  return forwardJsonPost(request, `/operator/auth/passkeys/${id}`, {
    sessionToken: request.auth?.user?.token ?? null,
    method: "DELETE",
    hasBody: false,
  });
}

export const DELETE = withOperatorAuth(DELETEHandler);
