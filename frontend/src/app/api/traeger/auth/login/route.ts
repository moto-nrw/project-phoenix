import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";

// Träger-Portal login (#4649). Static route wins over the NextAuth
// [...nextauth] catch-all — same coexistence as /api/school/auth/login.
// The backend answers either a token pair or the choice of Träger.
export async function POST(request: NextRequest) {
  return forwardJsonPost(request, "/traeger/auth/login");
}
