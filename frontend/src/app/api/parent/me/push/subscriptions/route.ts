import {
  createParentDeleteHandler,
  parentApiDelete,
  proxyPost,
} from "~/lib/parent/route-wrapper.server";

interface PushSubscriptionBody {
  endpoint: string;
  keys: { p256dh: string; auth: string };
}

/**
 * POST /api/parent/me/push/subscriptions — register this device for Web Push
 * across every school the guardian account is linked to (#2003).
 */
export const POST = proxyPost<null, PushSubscriptionBody>(
  "/parent/me/push/subscriptions",
);

/**
 * DELETE /api/parent/me/push/subscriptions?endpoint=… — remove this device's
 * registration across all linked schools.
 */
export const DELETE = createParentDeleteHandler<null>(
  async (request, token) => {
    const endpoint = request.nextUrl.searchParams.get("endpoint") ?? "";
    await parentApiDelete<null>(
      `/parent/me/push/subscriptions?endpoint=${encodeURIComponent(endpoint)}`,
      token,
    );
    return null;
  },
);
