import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { tenantPortal } from "~/server/auth";
import { createFileExportRoute } from "~/lib/export-proxy.server";

// Printable Dienstplan week (#2079) — a file download, so it bypasses the
// JSON route wrapper and streams the bytes through.
export const POST = withPrivateDownloadNoStore(
  tenantPortal.withAuth(
    createFileExportRoute({
      backendPath: "/api/staff-shifts/export",
    }),
  ),
);
