import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface BackendChild {
  student_id: string;
  tenant_id: string;
  first_name: string;
  last_name: string;
  school_class?: string;
  status: string;
  enrolled_from?: string;
  enrolled_until?: string;
  school_name: string;
  school_slug: string;
}

/**
 * Proxy GET /api/parent/me/children → backend /parent/me/children.
 * The route-wrapper handles parent-session auth + 401 retry with
 * refreshed token. Account id is derived server-side from the parent
 * JWT claims — never trust a client-provided account_id.
 */
export const GET = proxyGet<BackendChild[]>("/parent/me/children");
