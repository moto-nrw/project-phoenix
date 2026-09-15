// Anhänge einer Elternmitteilung (#2890), Elternseite: die Liste.
//
// Der Backend-Pfad liegt nicht unter /parent, weil das dort ein
// Catch-all-Mount ist; er ist wie der Eltern-SSE-Strom an der Wurzel gemountet
// und mit ParentMiddleware geschützt. Wer nicht im Empfängerkreis der
// Mitteilung ist, bekommt 404.

import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (p) =>
    `/parent-news-attachments/${requirePathSegmentParam(p, "announcementId")}`,
);
