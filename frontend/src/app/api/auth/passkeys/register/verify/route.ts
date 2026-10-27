import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

async function POSTHandler(request: NextRequest) {
  return forwardJsonPost(request, "/auth/passkeys/register/verify");
}

export const POST = withPrivateNoStore(POSTHandler);
