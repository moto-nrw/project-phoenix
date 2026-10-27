import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
// Einverständnis eines eigenen Elternbriefs in moto schule (#4800): den
// Nachweisbericht als PDF oder den Verlauf als CSV herunterladen. Derselbe
// Download wie im OGS-Portal (#3430), mit der Sitzung von moto schule.

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import {
  schoolAuth,
  uncachedSchoolAuth,
  withSchoolAuth,
} from "~/server/auth/school";
import { getServerApiUrl } from "~/lib/server-api-url";
import { incomingAnalyticsSessionHeaders } from "~/lib/analytics-session-header.server";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({
  component: "SchoolDeclarationExportDownload",
});

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

  const session = await schoolAuth();
  const token = session?.user?.token;
  if (!token) {
    return problemResponse(401, "Unauthorized", request);
  }

  const backendUrl = `${getServerApiUrl()}/school/parent-letters/${encodeURIComponent(announcementId)}/declaration-export?format=${format}`;

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
    const refreshed = await uncachedSchoolAuth();
    if (refreshed?.user?.token && refreshed.user.token !== token) {
      upstream = await makeRequest(refreshed.user.token);
    }
  }

  if (!upstream.ok) {
    logger.warn("school_declaration_export_proxy_non_ok", {
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

export const GET = withSchoolAuth(GETHandler);
