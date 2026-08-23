import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";

export async function POST(request: NextRequest) {
  return forwardJsonPost(request, "/school/auth/mfa/verify");
}
