import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { tenantPortal } from "~/server/auth";
import { createFileExportRoute } from "~/lib/export-proxy.server";
import { normalizeSlotListRequestBody } from "../request-normalizer";

// File download proxy — bypasses the JSON route wrapper and streams the
// PDF/XLSX bytes through.
export const POST = withPrivateDownloadNoStore(
  tenantPortal.withAuth(
    createFileExportRoute({
      backendPath: "/api/timetable/lists/export",
      normalizeBody: normalizeSlotListRequestBody,
    }),
  ),
);
