import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withOperatorAuth } from "~/server/auth/operator-route";
import type { ServerAuthRequest } from "~/server/auth/route-handler";

async function POSTHandler(request: ServerAuthRequest) {
  return forwardJsonPost(request, "/operator/auth/passkeys/register/options", {
    sessionToken: request.auth?.user?.token ?? null,
  });
}

export const POST = withOperatorAuth(POSTHandler);
