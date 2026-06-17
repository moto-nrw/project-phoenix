import { type NextRequest } from "next/server";
import { parentAuth } from "~/server/auth/parent";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentSSEEventsRoute" });

// REQUIRED for streaming - must use Node.js runtime
export const runtime = "nodejs";

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

/**
 * Parent-portal SSE proxy. Mirrors app/api/sse/events/route.ts but authenticates
 * with the parent NextAuth session (parent.session-token) and proxies to the
 * backend's parent-scoped stream at /parent-sse/events, which delivers only the
 * whitelisted parent_message trigger for the tenants of the guardian's children.
 *
 * Streaming bypasses the route wrapper; EventSource cannot set headers, so the
 * parent JWT is injected server-side before proxying to the Go backend.
 */
export async function GET(request: NextRequest) {
  const session = await parentAuth();

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
    const qs = request.nextUrl.search ? request.nextUrl.search : "";
    const backendResponse = await fetch(
      `${getServerApiUrl()}/parent-sse/events${qs}`,
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
      logger.error("parent SSE backend connection failed", {
        status: backendResponse.status,
        error: body,
      });
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

    return new Response(stream, {
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
        "X-Accel-Buffering": "no",
      },
    });
  } catch (error) {
    cleanup();

    if (request.signal.aborted || isAbortError(error)) {
      logger.debug("parent SSE proxy aborted", {
        aborted_by_client: request.signal.aborted,
      });
      return new Response(null, { status: 204 });
    }

    logger.error("parent SSE proxy error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return new Response("Internal server error", { status: 500 });
  }
}
