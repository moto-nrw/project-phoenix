import { proxyGet } from "~/lib/caterer/route-wrapper.server";
import type { BackendCatererOrders } from "~/lib/caterer-api";

/** One school's orders of a window, as its caterer reads them. */
export const GET = proxyGet<BackendCatererOrders>(
  (params) =>
    `/caterer/schools/${encodeURIComponent(String(params.schoolId))}/orders`,
);
