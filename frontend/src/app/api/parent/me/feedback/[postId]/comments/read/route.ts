import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface ReadBody {
  school_id: string;
}

/** Proxy POST /api/parent/me/feedback/{postId}/comments/read → backend. */
export const POST = proxyPost<{ read: boolean }, ReadBody>(
  (params) =>
    `/parent/me/feedback/${requirePathSegmentParam(params, "postId")}/comments/read`,
);
