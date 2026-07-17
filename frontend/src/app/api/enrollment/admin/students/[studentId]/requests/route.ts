import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentStudentRequestsRoute" });

interface RouteContext {
  params: Promise<{ studentId: string }>;
}

async function GETHandler(_request: NextRequest, context: RouteContext) {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }

  const { studentId } = await context.params;
  try {
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/admin/students/${encodeURIComponent(
        studentId,
      )}/requests`,
      {
        headers: { Authorization: `Bearer ${token}` },
        cache: "no-store",
      },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("student_enrollment_requests_get_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
