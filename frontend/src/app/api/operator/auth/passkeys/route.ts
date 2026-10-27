import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withOperatorAuth } from "~/server/auth/operator-route";
import type { ServerAuthRequest } from "~/server/auth/route-handler";

async function GETHandler(request: ServerAuthRequest) {
  return forwardJsonPost(request, "/operator/auth/passkeys/", {
    sessionToken: request.auth?.user?.token ?? null,
    method: "GET",
    hasBody: false,
  });
}

export const GET = withOperatorAuth(GETHandler);
