import { createProxyGetDataHandler } from "~/lib/route-wrapper.server";

// Tenant-wide list of vacation requests still awaiting decision. Used by the
// per-staff Abwesenheiten tab to count incoming requests, and (next step) by
// the inbox section on /staff.
export const GET = createProxyGetDataHandler("/api/staff/absences/pending");
