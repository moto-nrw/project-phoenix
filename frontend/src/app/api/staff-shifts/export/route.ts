import { withTenantAuth } from "~/server/auth/tenant-route";
import { createFileExportRoute } from "~/lib/export-proxy.server";

// Printable Dienstplan week (#2079) — a file download, so it bypasses the
// JSON route wrapper and streams the bytes through.
export const POST = withTenantAuth(
  createFileExportRoute({
    backendPath: "/api/staff-shifts/export",
    component: "DienstplanExportRoute",
  }),
);
