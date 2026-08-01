import { withTenantAuth } from "~/server/auth/tenant-route";
import { createFileExportRoute } from "~/lib/export-proxy.server";
import { normalizeSlotListRequestBody } from "../request-normalizer";

// File download proxy — bypasses the JSON route wrapper and streams the
// PDF/XLSX bytes through.
export const POST = withTenantAuth(
  createFileExportRoute({
    backendPath: "/api/timetable/lists/export",
    component: "SlotListExportRoute",
    normalizeBody: normalizeSlotListRequestBody,
  }),
);
