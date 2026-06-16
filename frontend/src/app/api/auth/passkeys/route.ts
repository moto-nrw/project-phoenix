import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy.server";

export async function GET(request: NextRequest) {
  return forwardJsonPost(request, "/auth/passkeys/", {
    method: "GET",
    hasBody: false,
  });
}
