import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthLinkToTenantRoute" });

async function POSTHandler(request: NextRequest) {
  try {
    const body = (await request.json()) as Record<string, unknown>;

    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json(
        { error: "Nicht authentifiziert" },
        { status: 401 },
      );
    }

    const response = await fetch(`${getServerApiUrl()}/auth/link-to-tenant`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${session.user.token}`,
      },
      body: JSON.stringify(body),
    });

    const contentType = response.headers.get("content-type") ?? "";
    let responseData: unknown = null;
    if (contentType.includes("application/json")) {
      responseData = await response.json();
    } else {
      const text = await response.text();
      responseData = text ? { error: text } : null;
    }

    if (!response.ok) {
      logger.error("link_to_tenant_failed", { status: response.status });
    }

    return NextResponse.json(responseData ?? { error: "Empty response" }, {
      status: response.status,
    });
  } catch (error) {
    logger.error("link_to_tenant_error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const POST = withTenantAuth(POSTHandler);
