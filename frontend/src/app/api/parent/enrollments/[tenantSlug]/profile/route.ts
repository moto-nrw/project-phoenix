import {
  createParentGetHandler,
  parentApiGet,
} from "~/lib/parent/route-wrapper.server";

interface ProfileGuardian {
  first_name: string;
  last_name: string;
  email: string;
  phone?: string;
}
interface ProfileChild {
  id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  grade_level?: number;
}
interface ProfileResponse {
  guardian: ProfileGuardian;
  children: ProfileChild[];
}

/**
 * Proxy GET /api/parent/enrollments/{tenantSlug}/profile →
 * backend /parent/enrollments/{tenantSlug}/profile. Forwards the parent
 * NextAuth session token; returns 401 (handled by the route wrapper)
 * when the parent isn't logged in. tenantSlug is taken straight from
 * the URL — backend resolves it under WithAdminTx.
 */
export const GET = createParentGetHandler<ProfileResponse>(
  async (_request, token, params) => {
    const slug = typeof params.tenantSlug === "string" ? params.tenantSlug : "";
    return parentApiGet<ProfileResponse>(
      `/parent/enrollments/${encodeURIComponent(slug)}/profile`,
      token,
    );
  },
);
