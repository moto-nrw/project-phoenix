import { type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffTimeTrackingExportRoute" });

// Admin export proxy. Streams CSV/XLSX binary from the backend as a file
// download. Mirrors /api/time-tracking/export but for a specific staff ID.
async function GETHandler(
  request: NextRequest,
  context: { params: Promise<{ id: string }> },
) {
  const session = await auth();
  if (!session?.user?.token) {
    return new Response("Unauthorized", { status: 401 });
  }

  const { id } = await context.params;
  if (!/^\d+$/.test(id)) {
    return new Response("Invalid staff id", { status: 400 });
  }

  try {
    const qs = request.nextUrl.search ?? "";
    const backendResponse = await fetch(
      `${getServerApiUrl()}/api/staff/${id}/time-tracking/export${qs}`,
      {
        headers: { Authorization: `Bearer ${session.user.token}` },
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
    logger.error("export_proxy_failed", {
      staff_id: id,
      error: error instanceof Error ? error.message : String(error),
    });
    return new Response("Internal server error", { status: 500 });
  }
}

export const GET = withTenantAuth(GETHandler);
