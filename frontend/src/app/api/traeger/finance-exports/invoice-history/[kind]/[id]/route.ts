import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Verlauf einer Rechnung: Ausstellung, Versand, Storno, Einzug und jeder
// Export (#4693), mit dem Recht „Abrechnung“.
export const GET = proxyGet(
  (params) =>
    `/traeger/finance-exports/invoice-history/${requirePathSegmentParam(params, "kind")}/${requirePathSegmentParam(params)}`,
);
