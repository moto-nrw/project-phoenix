import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffPreviewEndRoute" });

/**
 * POST /api/auth/staff-preview/end
 * Records the end of a staff-view preview for the audit trail (#2893).
 * The backend route is public and token-proved (the signed preview token in
 * the body is the credential); the session check here only keeps this proxy
 * from being an anonymous relay. The jwt callback's automatic endings call
 * the backend directly and never pass through this route.
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
    const response = await fetch(
      `${getServerApiUrl()}/auth/staff-preview/end`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(body),
      },
    );

    const data: unknown = await response.json();

    return NextResponse.json(data, { status: response.status });
  } catch (error) {
    logger.error("staff_preview_end_proxy_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const POST = withTenantAuth(POSTHandler);
