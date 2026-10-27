import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

// Caterer-portal login (ADR 0077). Static route wins over the NextAuth
// [...nextauth] catch-all, as for the school portal.
async function POSTHandler(request: NextRequest) {
  return forwardJsonPost(request, "/caterer/auth/login");
}

export const POST = withPrivateNoStore(POSTHandler);
