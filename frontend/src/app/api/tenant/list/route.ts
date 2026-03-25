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

    const contentType = response.headers.get("content-type") ?? "";
    if (!contentType.includes("application/json")) {
      const text = await response.text();
      logger.warn("tenant_list_non_json_response", {
        status: response.status,
        content_type: contentType,
        body: text.slice(0, 200),
      });
      return NextResponse.json(
        { error: text || response.statusText },
        { status: response.status },
      );
    }

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
