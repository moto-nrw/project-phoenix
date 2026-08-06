import { withTenantAuth } from "~/server/auth/tenant-route";
import { createFileExportRoute } from "~/lib/export-proxy.server";

export const POST = withTenantAuth(
  createFileExportRoute({
    backendPath: "/api/rooms/export",
    component: "RoomExportRoute",
  }),
);
