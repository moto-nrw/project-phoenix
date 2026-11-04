import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
// Fehlzeitenliste einer Klasse in moto schule (#4793) als PDF oder Excel
// herunterladen, mit der Sitzung von moto schule. Welche Klasse die Person
// sehen darf, prüft das Backend; es protokolliert jeden Export.

import type { NextRequest } from "next/server";
import {
  schoolAuth,
  uncachedSchoolAuth,
  withSchoolAuth,
} from "~/server/auth/school";
import { getServerApiUrl } from "~/lib/server-api-url";
import { incomingAnalyticsSessionHeaders } from "~/lib/analytics-session-header.server";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";
import { pdfDownloadResponse } from "~/lib/pdf-download-response.server";

const logger = createLogger({
  component: "SchoolAbsenceListExportDownload",
});

const FORMATS = new Set(["pdf", "xlsx"]);

function exportQuery(request: NextRequest): string {
  const params = new URLSearchParams();
  for (const key of ["class", "from", "to", "format"] as const) {
    const value = request.nextUrl.searchParams.get(key);
    if (value) params.set(key, value);
  }
  return params.toString();
}

async function GETHandler(request: NextRequest) {
  const format = request.nextUrl.searchParams.get("format") ?? "";
  if (!FORMATS.has(format)) {
    return problemResponse(400, "format must be pdf or xlsx", request);
  }

  const session = await schoolAuth();
  const token = session?.user?.token;
  if (!token) {
    return problemResponse(401, "Unauthorized", request);
  }

  const backendUrl = `${getServerApiUrl()}/school/class-day/absence-list/export?${exportQuery(request)}`;

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
    logger.warn("school_absence_list_export_proxy_non_ok", {
      format,
      status: upstream.status,
    });
    return forwardBackendResponse(upstream, request);
  }

  return pdfDownloadResponse(upstream);
}

export const GET = withPrivateDownloadNoStore(withSchoolAuth(GETHandler));
