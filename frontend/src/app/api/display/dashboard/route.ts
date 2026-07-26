import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DisplayDashboardRoute" });

/**
 * Public proxy for the info-point display dashboard. No session — the opaque
 * display token in the X-Display-Token header is the only auth signal; the
 * backend resolves tenant scope from it. The token travels as a header (never
 * in the URL) so it cannot leak into request-path logs at any hop.
 */
export async function GET(request: NextRequest) {
  const token = request.headers.get("x-display-token");
  if (!token) {
    return NextResponse.json({ error: "token required" }, { status: 400 });
  }
  try {
    const response = await fetch(`${getServerApiUrl()}/api/display/dashboard`, {
      cache: "no-store",
      headers: { "X-Display-Token": token },
    });
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
