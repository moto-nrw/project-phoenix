import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({
  component: "EnrollmentAdminOfferingAdjustmentsRoute",
});

async function bearerHeader() {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) return null;
  return `Bearer ${token}`;
}

async function GETHandler(request: NextRequest) {
  const authHeader = await bearerHeader();
  if (!authHeader) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }

  const requestID = request.nextUrl.searchParams.get("request_id") ?? "";
  const childID = request.nextUrl.searchParams.get("child_id") ?? "";
  if (!requestID || !childID) {
    return NextResponse.json(
      { error: "request_id and child_id are required" },
      { status: 400 },
    );
  }

  try {
    const upstream = await fetch(
      `${getServerApiUrl()}/api/enrollment/admin/requests/${encodeURIComponent(
        requestID,
      )}/children/${encodeURIComponent(childID)}/offering-adjustments`,
      {
        headers: { Authorization: authHeader },
        cache: "no-store",
      },
    );
    const payload = await upstream.json().catch(() => ({}));
    return NextResponse.json(payload, { status: upstream.status });
  } catch (error) {
    logger.error("admin_child_offering_adjustments_list_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
