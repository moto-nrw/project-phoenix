import { proxyDelete, proxyPut } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// PUT /api/school/class-day/arrival-exceptions/[schoolClass]/[date]:
// andere Ankunftszeit einer zugewiesenen Klasse an einem Tag setzen (#2970).
export const PUT = proxyPut(
  (p) =>
    `/school/class-day/arrival-exceptions/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "date")}`,
);

// DELETE: die Ausnahme wieder entfernen.
export const DELETE = proxyDelete(
  (p) =>
    `/school/class-day/arrival-exceptions/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "date")}`,
);
