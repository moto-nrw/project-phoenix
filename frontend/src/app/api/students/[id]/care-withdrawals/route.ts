import { proxyGet } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// Die abgeschlossenen Abmeldungen eines Kindes für sein Änderungsprotokoll
// (#5025). Anders als /api/students/care-withdrawals reicht users:update oder
// users:absence plus Lesezugriff auf das Kind.
export const GET = proxyGet(
  (params) =>
    `/api/students/${requirePathSegmentParam(params, "id")}/care-withdrawals`,
);
