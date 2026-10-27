import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

// School-portal login (#2207). Static route wins over the NextAuth
// [...nextauth] catch-all — same coexistence as /api/auth/login.
async function POSTHandler(request: NextRequest) {
  return forwardJsonPost(request, "/school/auth/login");
}

export const POST = withPrivateNoStore(POSTHandler);
