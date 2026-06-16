import {
  createParentGetHandler,
  createParentPostHandler,
  parentApiGet,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

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
export const GET = createParentGetHandler<BackendRelatedAccount[]>(
  async (_request, token, params) => {
    const studentId = String(params.studentId);
    return parentApiGet<BackendRelatedAccount[]>(
      `/parent/me/children/${encodeURIComponent(studentId)}/related-accounts`,
      token,
    );
  },
);

// POST /api/parent/me/children/{studentId}/related-accounts
// Invite a further guardian by email. Ownership + invite-mode gate are
// enforced server-side.
export const POST = createParentPostHandler<unknown, InviteBody>(
  async (_request, body, token, params) => {
    const studentId = String(params.studentId);
    return parentApiPost<unknown, InviteBody>(
      `/parent/me/children/${encodeURIComponent(studentId)}/related-accounts`,
      token,
      body,
    );
  },
);
