import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { withTenantAuth } from "~/server/auth";
import { createFileExportRoute } from "~/lib/export-proxy.server";

export const POST = withPrivateDownloadNoStore(
  withTenantAuth(
    createFileExportRoute({
      backendPath: "/api/guardians/payment-overview/export",
    }),
  ),
);
