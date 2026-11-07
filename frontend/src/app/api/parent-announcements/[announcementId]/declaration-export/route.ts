import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
// Erklärungen (#3430): den Nachweisbericht als PDF oder den Verlauf als CSV
// herunterladen. Beide Dateien erzeugt das Backend.
//
// Eigener Handler statt des JSON-Proxys, weil hier Bytes durchlaufen.

import type { NextRequest } from "next/server";
import { sendWithTokenRefresh } from "~/lib/token-refresh-retry.server";
import { NextResponse } from "next/server";
import { auth, uncachedAuth, withTenantAuth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { incomingAnalyticsSessionHeaders } from "~/lib/analytics-session-header.server";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "DeclarationExportDownload" });

const FORMATS = new Set(["csv", "pdf"]);

async function GETHandler(
  request: NextRequest,
  { params }: { params: Promise<{ announcementId: string }> },
) {
  const { announcementId } = await params;

  const format = request.nextUrl.searchParams.get("format") ?? "";
  if (!FORMATS.has(format)) {
    return problemResponse(400, "format must be csv or pdf", request);
  }

  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return problemResponse(401, "Unauthorized", request);
  }

  const backendUrl = `${getServerApiUrl()}/api/parent-announcements/${encodeURIComponent(announcementId)}/declaration-export?format=${format}`;

  const makeRequest = async (bearer: string) =>
    fetch(backendUrl, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${bearer}`,
        ...(await incomingAnalyticsSessionHeaders()),
      },
      cache: "no-store",
    });

  const upstream = await sendWithTokenRefresh(token, makeRequest, () =>
    uncachedAuth(),
  );

  if (!upstream.ok) {
    logger.warn("declaration_export_proxy_non_ok", {
      announcement_id: announcementId,
      format,
      status: upstream.status,
    });
    return forwardBackendResponse(upstream, request);
  }

  const headers = new Headers();
  for (const name of [
    "content-type",
    "content-disposition",
    "cache-control",
    "content-length",
    "x-content-type-options",
  ]) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }

  return new NextResponse(upstream.body, { status: 200, headers });
}

export const GET = withPrivateDownloadNoStore(withTenantAuth(GETHandler));
