// app/api/students/[id]/change-history/route.ts
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createLogger } from "~/lib/logger";
import { auth } from "~/server/auth";

const logger = createLogger({ component: "StudentChangeHistoryRoute" });

/**
 * Proxy for GET /api/students/[id]/change-history (issue #1455).
 *
 * The backend gates access on full access (admin / group supervisor) and
 * returns the per-child change history. We forward and unwrap the envelope.
 */
export async function GET(request: NextRequest): Promise<NextResponse> {
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

  try {
    const envelope = await apiGet<{ data: unknown }>(
      `/api/students/${studentId}/change-history`,
      session.user.token,
    );
    return NextResponse.json(
      { status: "success", data: envelope.data },
      { headers: { "Cache-Control": "no-store" } },
    );
  } catch (apiError) {
    const message =
      apiError instanceof Error ? apiError.message : String(apiError);

    const statusMatch = message.match(/API error \((\d+)\)/);
    const status = statusMatch?.[1] ? Number.parseInt(statusMatch[1], 10) : 500;

    if (status === 403) {
      return NextResponse.json({ error: "forbidden" }, { status: 403 });
    }
    if (status === 404) {
      return NextResponse.json({ error: "not_found" }, { status: 404 });
    }

    logger.error("change_history_fetch_failed", {
      student_id: studentId,
      error: message,
    });
    return NextResponse.json(
      { error: `Backend API error: ${message}` },
      { status },
    );
  }
}
