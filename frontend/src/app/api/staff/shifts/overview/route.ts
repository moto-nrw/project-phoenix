import { proxyGet } from "~/lib/route-proxy.server";
import type { BackendStaffScheduleOverview } from "~/lib/shift-helpers";

/**
 * GET /api/staff/shifts/overview?from=YYYY-MM-DD&to=YYYY-MM-DD
 * Read-only weekly bridge between planned shifts and concrete assignments.
 */
export const GET = proxyGet<BackendStaffScheduleOverview>(
  "/api/staff-shifts/overview",
);
