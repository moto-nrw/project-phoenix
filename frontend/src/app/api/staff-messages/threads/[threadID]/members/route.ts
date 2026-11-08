import { proxyPost } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Mitglieder zu einem Gruppenchat hinzufügen (#4776).
export const POST = proxyPost(
  (p) =>
    `/api/staff-messages/threads/${requirePathSegmentParam(p, "threadID")}/members`,
);
