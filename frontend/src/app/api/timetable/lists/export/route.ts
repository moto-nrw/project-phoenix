import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { withTenantAuth } from "~/server/auth";
import { createFileExportRoute } from "~/lib/export-proxy.server";
import { normalizeSlotListRequestBody } from "../request-normalizer";

// File download proxy — bypasses the JSON route wrapper and streams the
// PDF/XLSX bytes through.
export const POST = withPrivateDownloadNoStore(
  withTenantAuth(
    createFileExportRoute({
      backendPath: "/api/timetable/lists/export",
      normalizeBody: normalizeSlotListRequestBody,
    }),
  ),
);
