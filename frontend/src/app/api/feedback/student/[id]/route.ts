import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createLogger } from "~/lib/logger";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";

const logger = createLogger({ component: "StudentFeedbackRoute" });

interface BackendFeedbackEntry {
  id: number;
  value: "positive" | "neutral" | "negative";
  day: string;
  time: string;
  student_id: number;
  is_mensa_feedback: boolean;
  created_at: string;
  updated_at: string;
  student?: {
    id: number;
    first_name: string;
    last_name: string;
  };
}

interface BackendFeedbackResponse {
  status: string;
  data: BackendFeedbackEntry[];
  message: string;
}

/**
 * Proxy for GET /api/feedback/student/[id].
 *
 * The backend enforces the feature flag (`feedback.enabled`). When the
 * feature is disabled, it returns 403 with "feature_disabled". We forward
 * this error code so the frontend page can show the appropriate message.
 */
async function GETHandler(request: NextRequest): Promise<NextResponse> {
  const session = await auth();

  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const pathParts = request.nextUrl.pathname.split("/");
  const studentIndex = pathParts.indexOf("student");
  const studentId = studentIndex >= 0 ? pathParts[studentIndex + 1] : undefined;

  if (!studentId) {
    return NextResponse.json(
      { error: "Invalid id parameter" },
      { status: 400 },
    );
  }

  try {
    const response = await apiGet<BackendFeedbackResponse>(
      `/api/feedback/student/${studentId}`,
      session.user.token,
    );
    return NextResponse.json(
      { status: "success", data: response.data },
      { headers: { "Cache-Control": "no-store" } },
    );
  } catch (apiError) {
    const message =
      apiError instanceof Error ? apiError.message : String(apiError);

    const statusMatch = message.match(/API error \((\d+)\)/);
    const status = statusMatch?.[1] ? Number.parseInt(statusMatch[1], 10) : 500;

    if (status === 403) {
      return NextResponse.json({ error: "feature_disabled" }, { status: 403 });
    }
    if (status === 404) {
      return NextResponse.json({ error: "not_found" }, { status: 404 });
    }

    logger.error("feedback_fetch_failed", {
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
