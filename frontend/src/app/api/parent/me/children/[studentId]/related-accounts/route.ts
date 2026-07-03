import { proxyGet, proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendRelatedAccount {
  guardian_profile_id: string;
  first_name: string;
  last_name: string;
  email?: string;
  relationship_type: string;
  is_primary: boolean;
  status: string;
}

interface InviteBody {
  email: string;
  first_name?: string;
  last_name?: string;
}

// GET /api/parent/me/children/{studentId}/related-accounts
export const GET = proxyGet<BackendRelatedAccount[]>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/related-accounts`,
);

// POST /api/parent/me/children/{studentId}/related-accounts
// Invite a further guardian by email. Ownership + invite-mode gate are
// enforced server-side.
export const POST = proxyPost<unknown, InviteBody>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/related-accounts`,
);
