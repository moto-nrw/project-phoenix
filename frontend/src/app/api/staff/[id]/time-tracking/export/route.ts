import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { captureBffException } from "~/lib/sentry-bff.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "StaffTimeTrackingExportRoute" });

// Streams one person’s PDF/CSV/XLSX time report. The backend checks manage
// permission or time_tracking:own for the signed-in person’s staff ID.
async function GETHandler(
  request: NextRequest,
  context: { params: Promise<{ id: string }> },
) {
  const session = await auth();
  if (!session?.user?.token) {
    return problemResponse(401, "Unauthorized", request);
  }

  const { id } = await context.params;
  if (!/^\d+$/.test(id)) {
    return problemResponse(400, "Invalid staff id", request);
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

    if (!backendResponse.ok)
      return forwardBackendResponse(backendResponse, request);

    if (!backendResponse.body) {
      return problemResponse(502, "No response body from backend", request);
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
    captureBffException(error, request);
    logger.error("export_proxy_failed", {
      staff_id: id,
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal server error", request);
  }
}

export const GET = withPrivateDownloadNoStore(withTenantAuth(GETHandler));
