import { proxyGet, proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// GET /api/parent/me/children/{studentId}/guardians
// Lists the child's guardians with contact + pickup detail. Ownership is
// enforced server-side against the calling account's guardian links.
export const GET = proxyGet<unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/guardians`,
);

// POST /api/parent/me/children/{studentId}/guardians
// Adds an accountless contact. Permissions are enforced server-side.
export const POST = proxyPost<unknown, unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/guardians`,
);
