import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";

export const GET = withPrivateDownloadNoStore(
  proxyFile("/api/meal-plan/participants/week/export", {
    forwardQuery: true,
    fallbackType: "application/octet-stream",
    fallbackDisposition: "attachment",
  }),
);
