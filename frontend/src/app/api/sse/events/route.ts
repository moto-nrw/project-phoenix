import { type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { createLogger } from "~/lib/logger";
import { proxySSEStream } from "~/lib/sse-proxy.server";

const logger = createLogger({ component: "SSEEventsRoute" });

// REQUIRED for streaming - must use Node.js runtime
export const runtime = "nodejs";

/**
 * SSE (Server-Sent Events) proxy endpoint for the tenant/staff portal.
 *
 * Bypasses route-wrapper.ts because SSE requires streaming responses, not buffered
 * JSON. EventSource cannot set custom headers, so the JWT is injected server-side.
 * The streaming/abort/cleanup plumbing is shared with the parent portal via
 * proxySSEStream; only auth, the upstream path, and the logger differ here.
 */
async function GETHandler(request: NextRequest) {
  const session = await auth();

  if (!session?.user?.token) {
    return new Response("Unauthorized", { status: 401 });
  }

  return proxySSEStream(request, {
    token: session.user.token,
    upstreamPath: "/api/sse/events",
    logger,
  });
}

export const GET = withTenantAuth(GETHandler);
