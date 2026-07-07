import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface BackendEnrollmentRequestChild {
  child_id: string;
  first_name: string;
  last_name: string;
  status: string;
  status_reason?: string;
}

interface BackendEnrollmentRequest {
  request_id: string;
  tenant_id: string;
  status_token: string;
  submitted_at: string;
  withdrawn_at?: string;
  phase_id: string;
  phase_name: string;
  service_start_date: string;
  service_end_date: string;
  school_name: string;
  school_slug: string;
  children: BackendEnrollmentRequestChild[];
}

/**
 * Proxy GET /api/parent/me/enrollments → backend /parent/me/enrollments.
 * Returns every enrollment.requests row owned by the calling parent
 * (matched by guardian_account_id from the parent JWT). account_id is
 * read server-side from the JWT, never from query/body.
 */
export const GET = proxyGet<BackendEnrollmentRequest[]>(
  "/parent/me/enrollments",
);
