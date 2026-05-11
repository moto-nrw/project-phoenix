import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy";

export async function GET(request: NextRequest) {
  return forwardJsonPost(request, "/operator/auth/mfa/status", {
    method: "GET",
    hasBody: false,
  });
}
