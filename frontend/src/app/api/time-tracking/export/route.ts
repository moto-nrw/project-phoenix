import { type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TimeTrackingExportRoute" });

/**
 * Time-tracking export proxy endpoint
 * Streams CSV/XLSX binary from backend as a file download.
 *
 * This bypasses route-wrapper.ts because we need to stream binary data
 * with Content-Disposition headers, not buffered JSON.
 */
export async function GET(request: NextRequest) {
  const session = await auth();

  if (!session?.user?.token) {
    return new Response("Unauthorized", { status: 401 });
  }

  try {
    // Forward query params to backend
    const qs = request.nextUrl.search ?? "";
    const backendResponse = await fetch(
      `${getServerApiUrl()}/api/time-tracking/export${qs}`,
      {
        headers: {
          Authorization: `Bearer ${session.user.token}`,
        },
        cache: "no-store",
      },
    );

    if (!backendResponse.ok) {
      const body = await backendResponse.text().catch(() => "");
      return new Response(body || "Export failed", {
        status: backendResponse.status,
      });
    }

    if (!backendResponse.body) {
      return new Response("No response body from backend", { status: 502 });
    }

    // Pass through Content-Type and Content-Disposition from backend
    const headers = new Headers();
    const contentType = backendResponse.headers.get("Content-Type");
    const contentDisposition = backendResponse.headers.get(
      "Content-Disposition",
    );
    const contentLength = backendResponse.headers.get("Content-Length");

    if (contentType) headers.set("Content-Type", contentType);
    if (contentDisposition)
      headers.set("Content-Disposition", contentDisposition);
    if (contentLength) headers.set("Content-Length", contentLength);

    return new Response(backendResponse.body, { headers });
  } catch (error) {
    logger.error("export proxy error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return new Response("Internal server error", { status: 500 });
  }
}
