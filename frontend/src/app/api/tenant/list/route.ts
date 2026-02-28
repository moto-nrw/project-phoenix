import { NextResponse } from "next/server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TenantListRoute" });

/**
 * GET /api/tenant/list
 * Public endpoint (no auth required) — proxies to backend GET /auth/tenants.
 */
export async function GET() {
  try {
    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const response = await fetch(`${getServerApiUrl()}/auth/tenants`);

    const data: unknown = await response.json();

    return NextResponse.json(data, { status: response.status });
  } catch (error) {
    logger.error("tenant_list_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
