import { proxyGet } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Vergabeliste eines Ferienzeitraums nach Kriterien (#4700): je Woche die
// offenen Anmeldungen mit dem Vorschlag und der Verlauf der Entscheidungen.
export const GET = proxyGet(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/allocation`,
);
