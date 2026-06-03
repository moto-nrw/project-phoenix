import type { NextRequest } from "next/server";
import { forwardJsonPost } from "~/lib/auth-proxy";

export async function POST(request: NextRequest) {
  return forwardJsonPost(request, "/operator/auth/login");
}
