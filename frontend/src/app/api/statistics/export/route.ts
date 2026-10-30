// app/api/statistics/export/route.ts
import type { NextRequest } from "next/server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyStatisticsDownload } from "~/lib/statistics-download-proxy.server";
import { withTenantAuth } from "~/server/auth/tenant-route";

async function GETHandler(request: NextRequest) {
  return proxyStatisticsDownload(request, "/api/statistics/export", [
    "from",
    "to",
    "format",
    "section",
  ]);
}

export const GET = withPrivateDownloadNoStore(withTenantAuth(GETHandler));
