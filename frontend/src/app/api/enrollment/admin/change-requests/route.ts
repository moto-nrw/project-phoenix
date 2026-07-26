import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({
  component: "EnrollmentAdminChangeRequestsRoute",
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

  try {
    const url = new URL(
      `${getServerApiUrl()}/api/enrollment/admin/change-requests`,
    );
    const requestID = request.nextUrl.searchParams.get("request_id");
    const status = request.nextUrl.searchParams.get("status");
    if (requestID) url.searchParams.set("request_id", requestID);
    if (status) url.searchParams.set("status", status);

    const response = await fetch(url, {
      headers: { Authorization: authHeader },
      cache: "no-store",
    });
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("admin_change_requests_list_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
