import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentAdminListRoute" });

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
    const url = new URL(`${getServerApiUrl()}/api/enrollment/admin/requests`);
    const phase = request.nextUrl.searchParams.get("phase_id");
    const childStatus = request.nextUrl.searchParams.get("child_status");
    if (phase) url.searchParams.set("phase_id", phase);
    if (childStatus) url.searchParams.set("child_status", childStatus);
    const response = await fetch(url, {
      headers: { Authorization: authHeader },
      cache: "no-store",
    });
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("admin_requests_list_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
