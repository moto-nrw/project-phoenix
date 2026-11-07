import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/tenant-file-proxy.server";

export const GET = withPrivateDownloadNoStore(
  proxyFile("/api/students/day-log/export", {
    forwardQuery: ["date", "group_id", "format"],
  }),
);
