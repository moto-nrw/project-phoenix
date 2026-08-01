import { withTenantAuth } from "~/server/auth/tenant-route";
import { createFileExportRoute } from "~/lib/export-proxy.server";

// Printable Betreuungsplan week (#2079) — the care-plan counterpart of the
// Dienstplan export route.
export const POST = withTenantAuth(
  createFileExportRoute({
    backendPath: "/api/timetable/betreuungsplan/export",
    component: "BetreuungsplanExportRoute",
  }),
);
