import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Der Vertrag als PDF (#4678): vor dem Versand eine Vorschau, danach das
// gespeicherte Dokument.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/contracts/${requirePathSegmentParam(params)}/document`,
  ),
);
