import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface DeclarationSubmitResult {
  submission: unknown;
  created: boolean;
}

interface DeclarationSubmitBody {
  student_id?: string;
  action?: string;
  version_id?: string;
  password?: string;
}

/**
 * Proxy POST /api/parent/me/news/{announcementId}/declaration → backend.
 * Records one action of an Erklärung for ONE child (#3430); the backend
 * authorizes the child, checks the version, the deadline and, when asked
 * for, the account password. Password errors come back as 403 and pass
 * through unchanged, so they never trigger the 401 token refresh or logout.
 */
export const POST = proxyPost<DeclarationSubmitResult, DeclarationSubmitBody>(
  (params) =>
    `/parent/me/news/${requirePathSegmentParam(params, "announcementId")}/declaration`,
);
