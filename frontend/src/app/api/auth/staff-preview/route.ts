import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffPreviewRoute" });

/**
 * POST /api/auth/staff-preview
 * Mints a read-only staff-view preview token (#2893). Admin only —
 * the backend enforces the permission and the target's eligibility.
 */
async function POSTHandler(request: NextRequest) {
  try {
    const session = await auth();
    const token = session?.user?.token;

    if (!token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const body: unknown = await request.json();

    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const { getClientForwardHeaders } =
      await import("~/lib/client-headers.server");
    const response = await fetch(`${getServerApiUrl()}/auth/staff-preview`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
        // The start writes an audit event — the backend must see the admin's
        // real IP and browser, not the Docker-internal proxy hop.
        ...getClientForwardHeaders(request),
      },
      body: JSON.stringify(body),
    });

    const data: unknown = await response.json();

    return NextResponse.json(data, { status: response.status });
  } catch (error) {
    logger.error("staff_preview_start_proxy_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const POST = withTenantAuth(POSTHandler);
