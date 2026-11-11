import { proxyGet } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Angemeldete Kinder, die eine Schule des Trägers kennt (#6683): nur der
// Hinweis auf die Schule, jeder Abgleich steht im Protokoll.
export const GET = proxyGet(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/known-children`,
);
