import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

// Caterer passkey login (#4497); the frontend origin travels along so the
// backend can check the ceremony runs on the caterer host.
async function POSTHandler(request: NextRequest) {
  return forwardJsonPost(request, "/caterer/auth/passkeys/login/options");
}

export const POST = withPrivateNoStore(POSTHandler);
