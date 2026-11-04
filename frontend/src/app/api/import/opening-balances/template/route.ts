import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { captureBffException } from "~/lib/sentry-bff.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "OpeningBalanceTemplateRoute" });

async function GETHandler(request: NextRequest) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return problemResponse(401, "Unauthorized", request);
    }

    const searchParams = request.nextUrl.searchParams;
    const format = searchParams.get("format") ?? "csv";

    const response = await fetch(
      `${getServerApiUrl()}/api/import/opening-balances/template?format=${format}`,
      {
        headers: {
          Authorization: `Bearer ${session.user.token}`,
        },
      },
    );

    if (!response.ok) return forwardBackendResponse(response, request);

    const contentType =
      response.headers.get("Content-Type") ?? "text/csv; charset=utf-8";
    const contentDisposition =
      response.headers.get("Content-Disposition") ??
      'attachment; filename="eroeffnungssalden-import-vorlage.csv"';

    const blob = await response.blob();
    return new NextResponse(blob, {
      headers: {
        "Content-Type": contentType,
        "Content-Disposition": contentDisposition,
      },
    });
  } catch (error) {
    captureBffException(error, request);
    logger.error("template_download_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal server error", request);
  }
}

export const GET = withPrivateDownloadNoStore(withTenantAuth(GETHandler));
