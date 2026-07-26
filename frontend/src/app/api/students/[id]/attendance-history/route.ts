// app/api/students/[id]/attendance-history/route.ts
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createLogger } from "~/lib/logger";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";

const logger = createLogger({ component: "StudentAttendanceHistoryRoute" });

/**
 * Proxy for GET /api/students/[id]/attendance-history.
 *
 * The backend enforces the feature flag (`gdpr.attendance_log_enabled`),
 * scope (group supervisors vs. all staff), day-range clamping, and writes
 * the audit row. We just forward the request and surface the response
 * shape unchanged so the client can render error codes directly.
 */
async function GETHandler(request: NextRequest): Promise<NextResponse> {
  const session = await auth();

  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const pathParts = request.nextUrl.pathname.split("/");
  const studentsIndex = pathParts.indexOf("students");
  const studentId =
    studentsIndex >= 0 ? pathParts[studentsIndex + 1] : undefined;

  if (!studentId) {
    return NextResponse.json(
      { error: "Invalid id parameter" },
      { status: 400 },
    );
  }

  const queryParams = new URLSearchParams();
  const start = request.nextUrl.searchParams.get("start");
  const end = request.nextUrl.searchParams.get("end");
  if (start) queryParams.append("start", start);
  if (end) queryParams.append("end", end);

  const queryString = queryParams.toString();
  const querySuffix = queryString ? `?${queryString}` : "";
  const endpoint = `/api/students/${studentId}/attendance-history${querySuffix}`;

  try {
    // The backend wraps every response in `{ status, data, message }` via
    // common.Respond, and apiGet returns that envelope as-is. Unwrap the
    // inner payload once here so the client receives a single-level
    // `{ status, data }` shape.
    const envelope = await apiGet<{ data: unknown }>(
      endpoint,
      session.user.token,
    );
    return NextResponse.json(
      { status: "success", data: envelope.data },
      { headers: { "Cache-Control": "no-store" } },
    );
  } catch (apiError) {
    const message =
      apiError instanceof Error ? apiError.message : String(apiError);

    // apiGet throws errors in the format "API error (STATUS): body".
    // Extract the status code structurally instead of matching on body text.
    const statusMatch = message.match(/API error \((\d+)\)/);
    const status = statusMatch?.[1] ? Number.parseInt(statusMatch[1], 10) : 500;

    if (status === 403) {
      const code = message.includes("not_group_supervisor")
        ? "not_group_supervisor"
        : "feature_disabled";
      return NextResponse.json({ error: code }, { status: 403 });
    }
    if (status === 404) {
      return NextResponse.json({ error: "not_found" }, { status: 404 });
    }

    logger.error("attendance_history_fetch_failed", {
      student_id: studentId,
      error: message,
    });
    return NextResponse.json(
      { error: `Backend API error: ${message}` },
      { status },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
