import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
// Erklärungen (#3430): den Nachweisbericht als PDF oder den Verlauf als CSV
// herunterladen. Beide Dateien erzeugt das Backend.
//
// Eigener Handler statt des JSON-Proxys, weil hier Bytes durchlaufen. Spiegelt
// den Download-Proxy der Anhänge: JWT serverseitig setzen, bei 401 einmal
// erneuern, Body mit Content-Type und Content-Disposition unverändert
// durchreichen.

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { incomingAnalyticsSessionHeaders } from "~/lib/analytics-session-header.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DeclarationExportDownload" });

const FORMATS = new Set(["csv", "pdf"]);

async function GETHandler(
  request: NextRequest,
  { params }: { params: Promise<{ announcementId: string }> },
) {
  const { announcementId } = await params;

  const format = request.nextUrl.searchParams.get("format") ?? "";
  if (!FORMATS.has(format)) {
    return NextResponse.json(
      { status: "error", error: "format must be csv or pdf" },
      { status: 400 },
    );
  }

  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
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

  let upstream = await makeRequest(token);
  if (upstream.status === 401) {
    const refreshed = await uncachedAuth();
    if (refreshed?.user?.token && refreshed.user.token !== token) {
      upstream = await makeRequest(refreshed.user.token);
    }
  }

  if (!upstream.ok) {
    logger.warn("declaration_export_proxy_non_ok", {
      announcement_id: announcementId,
      format,
      status: upstream.status,
    });
    return forwardBackendResponse(upstream);
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

export const GET = withTenantAuth(GETHandler);
