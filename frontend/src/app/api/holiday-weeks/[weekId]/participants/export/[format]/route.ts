import { withTenantAuth } from "~/server/auth/tenant-route";
import { createFileDownloadRoute } from "~/lib/export-proxy.server";

/**
 * Proxy GET /api/holiday-weeks/{weekId}/participants/export?format=… →
 * backend (#4665): die Teilnehmerliste als CSV oder Excel. Das Format steht
 * im Pfad, weil die Download-Route keine Suchparameter weiterreicht; das
 * Backend lehnt jedes andere Format ab.
 */
export const GET = withTenantAuth(
  createFileDownloadRoute<{ weekId: string; format: string }>({
    backendPath: ({ weekId, format }) =>
      `/api/holiday-weeks/${encodeURIComponent(weekId)}/participants/export?format=${encodeURIComponent(format)}`,
    component: "HolidayParticipantsExportRoute",
  }),
);
