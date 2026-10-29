import { proxyDelete } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ein Mitglied aus einem Gruppenchat entfernen (#4776).
export const DELETE = proxyDelete(
  (p) =>
    `/api/staff-messages/threads/${requirePathSegmentParam(p, "threadID")}/members/${requirePathSegmentParam(p, "accountID")}`,
);
