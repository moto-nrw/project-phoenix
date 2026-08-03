import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OpeningBalanceTemplateRoute" });

async function GETHandler(request: NextRequest) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
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

    if (!response.ok) {
      const error = await response.text();
      return NextResponse.json(
        { error: error || "Failed to download template" },
        { status: response.status },
      );
    }

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
    logger.error("template_download_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
