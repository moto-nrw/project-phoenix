import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Das PDF einer Vertragsversion (#4681): das versendete Dokument, sonst die
// Fassung mit dem aktuellen Briefkopf.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/contracts/${requirePathSegmentParam(params)}/versions/${requirePathSegmentParam(params, "number")}/document`,
  ),
);
