import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";

// School-portal login (#2207). Static route wins over the NextAuth
// [...nextauth] catch-all — same coexistence as /api/auth/login.
export async function POST(request: NextRequest) {
  return forwardJsonPost(request, "/school/auth/login");
}
