import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { tenantPortal } from "~/server/auth";
import { createFileExportRoute } from "~/lib/export-proxy.server";

export const POST = withPrivateDownloadNoStore(
  tenantPortal.withAuth(
    createFileExportRoute<{ studentId: string }>({
      backendPath: ({ studentId }) =>
        `/api/enrollment/admin/students/${encodeURIComponent(studentId)}/requests/export`,
    }),
  ),
);
