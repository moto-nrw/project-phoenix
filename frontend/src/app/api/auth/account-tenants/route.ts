import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AccountTenantsRoute" });

/**
 * GET /api/auth/account-tenants
 * Returns the list of tenants the current user has access to.
 */
async function GETHandler(_request: NextRequest) {
  try {
    const session = await auth();
    const token = session?.user?.token;

    if (!token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const response = await fetch(`${getServerApiUrl()}/auth/account/tenants`, {
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      return NextResponse.json(
        { error: errorText },
        { status: response.status },
      );
    }

    const data: unknown = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    logger.error("list_account_tenants_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
