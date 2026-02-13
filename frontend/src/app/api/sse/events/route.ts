import { type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SSEEventsRoute" });

// REQUIRED for streaming - must use Node.js runtime
export const runtime = "nodejs";

/**
 * SSE (Server-Sent Events) proxy endpoint
 * Streams real-time updates from backend to browser
 *
 * This endpoint bypasses route-wrapper.ts because SSE requires streaming responses,
 * not buffered JSON responses. EventSource API cannot set custom headers, so we inject
 * the JWT token server-side before proxying to the Go backend.
 *
 * Note: Node.js 18+ includes native fetch with undici, which handles long-lived
 * connections appropriately. No need for explicit timeout configuration.
 */
export async function GET(request: NextRequest) {
  // Validate session
  const session = await auth();

  if (!session?.user?.token) {
    return new Response("Unauthorized", { status: 401 });
  }

  try {
    // Fetch SSE stream from Go backend with JWT token
    // Preserve query params (e.g., cache busters) though backend ignores them
    const qs = request.nextUrl.search ? request.nextUrl.search : "";
    const backendResponse = await fetch(
      `${getServerApiUrl()}/api/sse/events${qs}`,
      {
        headers: {
          Authorization: `Bearer ${session.user.token}`,
          Accept: "text/event-stream",
        },
        cache: "no-store",
      },
    );

    if (!backendResponse.ok) {
      const body = await backendResponse.text().catch(() => "");
      logger.error("SSE backend connection failed", {
        status: backendResponse.status,
        error: body,
      });
      // Propagate backend status to client for accurate diagnostics (e.g., 401/403)
      return new Response(body || "SSE connection failed", {
        status: backendResponse.status,
      });
    }

    if (!backendResponse.body) {
      return new Response("No response body from backend", { status: 502 });
    }

    // Stream backend SSE response to browser
    return new Response(backendResponse.body, {
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
        // Disable buffering for immediate event delivery
        "X-Accel-Buffering": "no",
      },
    });
  } catch (error) {
    logger.error("SSE proxy error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return new Response("Internal server error", { status: 500 });
  }
}
