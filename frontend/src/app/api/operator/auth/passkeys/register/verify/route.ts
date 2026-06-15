import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";

export async function POST(request: NextRequest) {
  return forwardJsonPost(request, "/operator/auth/passkeys/register/verify");
}
