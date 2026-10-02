import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy GET /api/parent/me/news/{announcementId}/declaration/proof?student_id=X
 * → backend (#3430). The proof data of an Erklärung for this account and one
 * child: the declared versions with full text and checksums, and the own
 * history. The portal renders it as a printable page; the backend answers
 * 404 without access or without an own submission yet.
 */
export const GET = proxyGet(
  (params) =>
    `/parent/me/news/${requirePathSegmentParam(params, "announcementId")}/declaration/proof`,
);
