import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { tenantPortal } from "~/server/auth";
import { createFileExportRoute } from "~/lib/export-proxy.server";

export const POST = withPrivateDownloadNoStore(
  tenantPortal.withAuth(
    createFileExportRoute({
      backendPath: "/api/guardians/payment-overview/export",
    }),
  ),
);
