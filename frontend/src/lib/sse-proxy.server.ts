import { type NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { type Logger } from "~/lib/logger";

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

export interface SSEProxyOptions {
  /** Bearer token injected server-side (EventSource cannot set headers). */
  readonly token: string;
  /** Backend SSE path appended to getServerApiUrl(), e.g. "/api/sse/events". */
  readonly upstreamPath: string;
  /** Per-portal logger so log lines keep their component name. */
  readonly logger: Logger;
}

/**
 * Shared Server-Sent-Events proxy for the tenant and parent portals. Both portal
 * routes bypass the route wrapper (streaming, not buffered JSON) and inject their
 * JWT server-side; only the auth function, the upstream path, and the logger name
 * differ, so all of the load-bearing streaming / abort / cleanup plumbing lives
 * here once. A reader-leak or missed-cleanup fix made here applies to both portals
 * instead of having to be mirrored (and risk diverging) across two ~150-line files.
 *
 * The caller authenticates, extracts the token, and delegates here:
 *
 *   const session = await auth();
 *   if (!session?.user?.token) return new Response("Unauthorized", { status: 401 });
 *   return proxySSEStream(request, { token: session.user.token, upstreamPath, logger });
 */
export async function proxySSEStream(
  request: NextRequest,
  { token, upstreamPath, logger }: SSEProxyOptions,
): Promise<Response> {
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
    // Preserve query params (e.g. cache busters) though the backend ignores them.
    const qs = request.nextUrl.search ? request.nextUrl.search : "";
    const backendResponse = await fetch(
      `${getServerApiUrl()}${upstreamPath}${qs}`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
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
      // Propagate backend status to client for accurate diagnostics (e.g. 401/403).
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
        // Disable buffering for immediate event delivery.
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
