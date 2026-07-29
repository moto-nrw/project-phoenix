import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface BackendFeedbackSchool {
  school_id: string;
  school_name: string;
}

/**
 * Proxy GET /api/parent/me/feedback/schools → backend. The schools whose
 * feedback board the guardian may use, derived server-side from their own
 * children; the client never picks a school the backend has not vouched for.
 */
export const GET = proxyGet<BackendFeedbackSchool[]>(
  "/parent/me/feedback/schools",
);
