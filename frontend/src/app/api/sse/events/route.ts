import { type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SSEEventsRoute" });

// REQUIRED for streaming - must use Node.js runtime
export const runtime = "nodejs";

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

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

  const upstreamController = new AbortController();
  let cleanedUp = false;

  const cleanup = () => {
    if (cleanedUp) return;
    cleanedUp = true;
    request.signal.removeEventListener("abort", handleClientAbort);
  };

  const handleClientAbort = () => {
    upstreamController.abort();
  };

  if (request.signal.aborted) {
    upstreamController.abort();
  } else {
    request.signal.addEventListener("abort", handleClientAbort, { once: true });
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
        signal: upstreamController.signal,
      },
    );

    if (!backendResponse.ok) {
      cleanup();
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
      cleanup();
      return new Response("No response body from backend", { status: 502 });
    }

    const reader = backendResponse.body.getReader();
    let downstreamCancelled = false;

    const stream = new ReadableStream<Uint8Array>({
      async pull(controller) {
        try {
          const { done, value } = await reader.read();

          if (done) {
            cleanup();
            if (!downstreamCancelled) {
              controller.close();
            }
            return;
          }

          if (value && !downstreamCancelled) {
            controller.enqueue(value);
          }
        } catch (error) {
          cleanup();

          if (
            downstreamCancelled ||
            request.signal.aborted ||
            isAbortError(error)
          ) {
            return;
          }

          controller.error(error);
        }
      },
      async cancel() {
        downstreamCancelled = true;
        cleanup();
        upstreamController.abort();

        try {
          await reader.cancel();
        } catch {
          // Upstream may already be closed when the client disconnects.
        }
      },
    });

    // Stream backend SSE response to browser
    return new Response(stream, {
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
        // Disable buffering for immediate event delivery
        "X-Accel-Buffering": "no",
      },
    });
  } catch (error) {
    cleanup();

    if (request.signal.aborted || isAbortError(error)) {
      logger.debug("SSE proxy aborted", {
        aborted_by_client: request.signal.aborted,
      });
      return new Response(null, { status: 204 });
    }

    logger.error("SSE proxy error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return new Response("Internal server error", { status: 500 });
  }
}
