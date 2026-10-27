import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

// The second step of a caterer login with two-factor sign-in (#4497). The
// backend's trusted-device cookie passes through on "remember_device".
async function POSTHandler(request: NextRequest) {
  return forwardJsonPost(request, "/caterer/auth/mfa/verify");
}

export const POST = withPrivateNoStore(POSTHandler);
