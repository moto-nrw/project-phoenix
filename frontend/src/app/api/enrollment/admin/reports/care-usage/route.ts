import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentCareUsageReportRoute" });

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
      `${getServerApiUrl()}/api/enrollment/admin/reports/care-usage`,
    );
    for (const key of [
      "phase_id",
      "status",
      "care_offering_id",
      "grade_level",
      "day_count",
      "weekday",
      "pickup_time",
      "search",
    ]) {
      const value = request.nextUrl.searchParams.get(key);
      if (value) url.searchParams.set(key, value);
    }
    for (const value of request.nextUrl.searchParams.getAll(
      "care_offering_ids",
    )) {
      url.searchParams.append("care_offering_ids", value);
    }
    const response = await fetch(url, {
      headers: { Authorization: authHeader },
      cache: "no-store",
    });
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("care_usage_report_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
