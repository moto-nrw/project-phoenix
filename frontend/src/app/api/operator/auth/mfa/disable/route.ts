import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy";

export async function DELETE(request: NextRequest) {
  return forwardJsonPost(request, "/operator/auth/mfa", {
    method: "DELETE",
    hasBody: false,
  });
}
