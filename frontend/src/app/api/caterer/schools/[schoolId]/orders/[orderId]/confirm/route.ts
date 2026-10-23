import { proxyPost } from "~/lib/caterer/route-wrapper.server";

/** The caterer confirms that it received a fixed order. */
export const POST = proxyPost<void>(
  (params) =>
    `/caterer/schools/${encodeURIComponent(String(params.schoolId))}/orders/${encodeURIComponent(String(params.orderId))}/confirm`,
);
