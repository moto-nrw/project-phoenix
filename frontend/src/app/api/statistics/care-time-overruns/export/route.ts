// app/api/statistics/care-time-overruns/export/route.ts
import type { NextRequest } from "next/server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyStatisticsDownload } from "~/lib/statistics-download-proxy.server";
import { withTenantAuth } from "~/server/auth/tenant-route";

/** PDF/Excel export of the Soll-und-Ist overrun list (#4774). */
async function GETHandler(request: NextRequest) {
  return proxyStatisticsDownload(
    request,
    "/api/statistics/care-time-overruns/export",
    ["from", "to", "format"],
  );
}

export const GET = withPrivateDownloadNoStore(withTenantAuth(GETHandler));
