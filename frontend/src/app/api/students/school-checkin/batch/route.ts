import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentsSchoolCheckinBatchRoute" });

/**
 * POST /api/students/school-checkin/batch
 *
 * Proxy to the backend batch school-checkin handler (#2359). The backend
 * enforces the users:checkin permission, a verified staff identity, and the
 * web-attendance setting; this route is a thin pass-through.
 *
 * Request body: { action: "in" | "out", student_ids: number[] }
 * Response:     { action, results: [...], succeeded, failed }
 */
export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    const parsed =
      body && typeof body === "object"
        ? (body as { action?: unknown; student_ids?: unknown })
        : {};

    logger.info("school_checkin_batch_proxy", {
      action: typeof parsed.action === "string" ? parsed.action : "unknown",
      student_count: Array.isArray(parsed.student_ids)
        ? parsed.student_ids.length
        : 0,
    });

    const response = await apiPost(
      "/api/students/school-checkin/batch",
      token,
      body,
    );

    // @ts-expect-error — apiPost returns unknown; backend envelope is { data }
    return response.data;
  },
);
