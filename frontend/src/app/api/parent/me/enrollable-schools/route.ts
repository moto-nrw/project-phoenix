import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface BackendEnrollablePhase {
  school_id: string;
  school_name: string;
  school_slug: string;
  phase_id: string;
  phase_name: string;
  phase_kind: "school_year" | "holiday" | "custom";
  service_start_date: string;
  service_end_date: string;
  enrollment_open_at?: string;
  enrollment_close_at?: string;
  already_linked: boolean;
  audience: "open" | "new_students" | "linked_parents";
}

/**
 * Proxy GET /api/parent/me/enrollable-schools → backend
 * /parent/me/enrollable-schools. Returns the enrollment phases the calling
 * parent (matched by account_id from the parent JWT) may actually apply to,
 * pre-filtered and sorted server-side. account_id is read from the JWT,
 * never from query/body.
 */
export const GET = proxyGet<BackendEnrollablePhase[]>(
  "/parent/me/enrollable-schools",
);
