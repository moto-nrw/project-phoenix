import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentRolloverPreviewRoute" });

interface RouteContext {
  params: Promise<{ id: string }>;
}

async function GETHandler(request: NextRequest, context: RouteContext) {
  const { id } = await context.params;
  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }
  try {
    const bumpsGrade =
      request.nextUrl.searchParams.get("bumps_grade") === "false"
        ? "false"
        : "true";
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/phases/${encodeURIComponent(id)}/rollover-preview?bumps_grade=${bumpsGrade}`,
      {
        headers: { Authorization: `Bearer ${token}` },
        cache: "no-store",
      },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("rollover_preview_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
