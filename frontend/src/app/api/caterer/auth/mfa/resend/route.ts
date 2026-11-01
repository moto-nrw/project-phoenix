import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

async function POSTHandler(request: NextRequest) {
  return forwardJsonPost(request, "/caterer/auth/mfa/resend");
}

export const POST = withPrivateNoStore(POSTHandler);
