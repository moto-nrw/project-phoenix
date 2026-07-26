import { type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffTimeExportRoute" });

/**
 * Cross-staff time-tracking export proxy (#1417 2b).
 * Streams the CSV/XLSX download from the backend, mirroring
 * /api/time-tracking/export: binary body plus Content-Disposition, so it
 * bypasses the JSON route wrapper.
 */
async function GETHandler(request: NextRequest) {
  const session = await auth();

  if (!session?.user?.token) {
    return new Response("Unauthorized", { status: 401 });
  }

  try {
    const qs = request.nextUrl.search ?? "";
    const backendResponse = await fetch(
      `${getServerApiUrl()}/api/staff/time-tracking/export${qs}`,
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
    logger.error("staff time export proxy error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return new Response("Internal server error", { status: 500 });
  }
}

export const GET = withTenantAuth(GETHandler);
