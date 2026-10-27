import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

async function GETHandler(request: NextRequest) {
  return forwardJsonPost(request, "/auth/mfa/trusted-devices", {
    method: "GET",
    hasBody: false,
  });
}

export const GET = withPrivateNoStore(GETHandler);
