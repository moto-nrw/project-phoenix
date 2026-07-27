import { apiDelete } from "~/lib/api-helpers.server";
import { proxyPost } from "~/lib/route-proxy.server";
import { createDeleteHandler } from "~/lib/route-wrapper.server";

interface PushSubscriptionBody {
  endpoint: string;
  keys: { p256dh: string; auth: string };
}

/**
 * POST /api/notifications/push/subscriptions — register this device for Web
 * Push (#2003). Body is the browser's PushSubscription JSON.
 */
export const POST = proxyPost<null, PushSubscriptionBody>(
  "/api/notifications/push/subscriptions",
);

/**
 * DELETE /api/notifications/push/subscriptions?endpoint=… — remove this
 * device's registration. The endpoint travels as a query parameter because
 * DELETE bodies don't survive every proxy layer.
 */
export const DELETE = createDeleteHandler<null>(async (request, token) => {
  const endpoint = request.nextUrl.searchParams.get("endpoint") ?? "";
  await apiDelete(
    `/api/notifications/push/subscriptions?endpoint=${encodeURIComponent(endpoint)}`,
    token,
  );
  return null;
});
