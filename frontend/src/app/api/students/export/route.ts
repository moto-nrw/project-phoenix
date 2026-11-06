import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/tenant-file-proxy.server";

export const POST = withPrivateDownloadNoStore(
  proxyFile("/api/students/export", { method: "POST" }),
);
