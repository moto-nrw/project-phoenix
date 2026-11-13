import { proxyGet } from "~/lib/route-wrapper.server";

// Wie viele Beschäftigte im Monat kein Soll haben (#6890), für /payroll.
export const GET = proxyGet("/api/staff/time-tracking/target-coverage", {
  forwardQuery: ["year", "month"],
});
