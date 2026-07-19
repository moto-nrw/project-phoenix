import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({
  component: "EnrollmentAdminChangeRequestDetailRoute",
});

interface RouteContext {
  params: Promise<{ id: string }>;
}

async function bearerHeader() {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) return null;
  return `Bearer ${token}`;
}

async function GETHandler(_request: NextRequest, context: RouteContext) {
  const { id } = await context.params;
  const authHeader = await bearerHeader();
  if (!authHeader) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }

  try {
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/admin/change-requests/${encodeURIComponent(
        id,
      )}`,
      { headers: { Authorization: authHeader }, cache: "no-store" },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("admin_change_request_get_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
