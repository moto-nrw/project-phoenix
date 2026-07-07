import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DisplayDashboardRoute" });

interface RouteContext {
  params: Promise<{ token: string }>;
}

/**
 * Public proxy for the info-point display dashboard. No session — the opaque
 * display token in the path is the only auth signal; the backend resolves
 * tenant scope from it (mirrors the enrollment status-token route).
 */
export async function GET(_request: NextRequest, context: RouteContext) {
  const { token } = await context.params;
  if (!token) {
    return NextResponse.json({ error: "token required" }, { status: 400 });
  }
  try {
    const response = await fetch(
      `${getServerApiUrl()}/api/display/${encodeURIComponent(token)}/dashboard`,
      { cache: "no-store" },
    );
    const payload = (await response.json().catch(() => ({}))) as unknown;
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("display_dashboard_fetch_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
