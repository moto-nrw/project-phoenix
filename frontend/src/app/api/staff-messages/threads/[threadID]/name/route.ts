import { proxyPut } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Gruppenchat umbenennen (#4776): nur wer angelegt hat und die Leitung.
export const PUT = proxyPut(
  (p) =>
    `/api/staff-messages/threads/${requirePathSegmentParam(p, "threadID")}/name`,
);
