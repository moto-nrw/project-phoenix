import {
  createSchoolDeleteHandler,
  proxyPost,
  schoolApiDelete,
} from "~/lib/school/route-wrapper.server";

interface PushSubscriptionBody {
  endpoint: string;
  keys: { p256dh: string; auth: string };
}

// Dieses Gerät für Push registrieren (#2208). Das Backend legt das Abo mit
// portal "school" ab.
export const POST = proxyPost<null, PushSubscriptionBody>(
  "/school/notifications/push/subscriptions",
);

// Abmelden: der Endpoint reist als Query-Parameter, weil DELETE-Bodies nicht
// jede Proxy-Schicht überleben (wie im OGS- und Elternportal).
export const DELETE = createSchoolDeleteHandler<null>(
  async (request, token) => {
    const endpoint = request.nextUrl.searchParams.get("endpoint") ?? "";
    await schoolApiDelete(
      `/school/notifications/push/subscriptions?endpoint=${encodeURIComponent(endpoint)}`,
      token,
    );
    return null;
  },
);
