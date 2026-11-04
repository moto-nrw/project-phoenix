import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { createFileExportRoute } from "~/lib/export-proxy.server";

export const POST = withPrivateDownloadNoStore(
  withTenantAuth(
    createFileExportRoute({
      backendPath: "/api/rooms/export",
      component: "RoomExportRoute",
    }),
  ),
);
