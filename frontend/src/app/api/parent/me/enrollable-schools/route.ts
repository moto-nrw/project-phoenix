import {
  createParentGetHandler,
  parentApiGet,
} from "~/lib/parent/route-wrapper";

interface BackendEnrollablePhase {
  school_id: string;
  school_name: string;
  school_slug: string;
  phase_id: string;
  phase_name: string;
  phase_kind: string;
  service_start_date: string;
  service_end_date: string;
  enrollment_open_at?: string;
  enrollment_close_at?: string;
  already_linked: boolean;
}

/**
 * Proxy GET /api/parent/me/enrollable-schools → backend
 * /parent/me/enrollable-schools. Returns every (school, open phase)
 * the parent can submit a new enrollment to. account_id is read
 * server-side from the parent JWT, never from query/body.
 */
export const GET = createParentGetHandler<BackendEnrollablePhase[]>(
  async (_request, token) =>
    parentApiGet<BackendEnrollablePhase[]>(
      "/parent/me/enrollable-schools",
      token,
    ),
);
