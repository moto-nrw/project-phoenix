import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";

// Caterer-portal login (ADR 0075). Static route wins over the NextAuth
// [...nextauth] catch-all, as for the school portal.
export async function POST(request: NextRequest) {
  return forwardJsonPost(request, "/caterer/auth/login");
}
