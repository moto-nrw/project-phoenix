import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentsSchoolCheckinRoute" });

/**
 * POST /api/students/[id]/school-checkin
 *
 * Proxy to the backend school-checkin handler. The backend enforces the
 * users:checkin permission and a verified staff identity (#2329); this route
 * is a thin pass-through.
 *
 * Request body: { action: "in" | "out" }
 * Response:     { status, location, check_in_time, check_out_time, ... }
 */
export const POST = createPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    if (!id) {
      throw new Error("Student ID is required");
    }

    logger.info("school_checkin_proxy", {
      student_id: id,
      action:
        body && typeof body === "object" && "action" in body
          ? String((body as { action?: unknown }).action)
          : "unknown",
    });

    const response = await apiPost(
      `/api/students/${id}/school-checkin`,
      token,
      body,
    );

    // @ts-expect-error — apiPost returns unknown; backend envelope is { data }
    return response.data;
  },
);
