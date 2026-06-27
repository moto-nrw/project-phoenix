import { type NextRequest } from "next/server";
import { parentAuth } from "~/server/auth/parent";
import { createLogger } from "~/lib/logger";
import { proxySSEStream } from "~/lib/sse-proxy.server";

const logger = createLogger({ component: "ParentSSEEventsRoute" });

// REQUIRED for streaming - must use Node.js runtime
export const runtime = "nodejs";

/**
 * Parent-portal SSE proxy. Same streaming proxy as the tenant route
 * (proxySSEStream), but authenticates with the parent NextAuth session
 * (parent.session-token) and proxies to the backend's parent-scoped stream at
 * /parent-sse/events, which delivers only the whitelisted parent_message trigger
 * for the tenants of the guardian's children.
 */
export async function GET(request: NextRequest) {
  const session = await parentAuth();

  if (!session?.user?.token) {
    return new Response("Unauthorized", { status: 401 });
  }

  return proxySSEStream(request, {
    token: session.user.token,
    upstreamPath: "/parent-sse/events",
    logger,
  });
}
