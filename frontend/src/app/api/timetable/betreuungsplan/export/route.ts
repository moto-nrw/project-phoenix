import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { tenantPortal } from "~/server/auth";
import { createFileExportRoute } from "~/lib/export-proxy.server";

// Printable Betreuungsplan week (#2079) — the care-plan counterpart of the
// Dienstplan export route.
export const POST = withPrivateDownloadNoStore(
  tenantPortal.withAuth(
    createFileExportRoute({
      backendPath: "/api/timetable/betreuungsplan/export",
    }),
  ),
);
