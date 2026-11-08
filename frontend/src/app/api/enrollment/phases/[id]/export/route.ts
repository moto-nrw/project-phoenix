import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { tenantPortal } from "~/server/auth";
import { createFileExportRoute } from "~/lib/export-proxy.server";

export const POST = withPrivateDownloadNoStore(
  tenantPortal.withAuth(
    createFileExportRoute<{ id: string }>({
      backendPath: ({ id }) =>
        `/api/enrollment/phases/${encodeURIComponent(id)}/export`,
    }),
  ),
);
