import { proxyGet } from "~/lib/route-proxy.server";

interface PublicKeyResponse {
  public_key: string;
}

/**
 * GET /api/notifications/push/public-key — VAPID public key for Web Push
 * subscription (#2003). 404 while the server has no VAPID keys configured.
 */
export const GET = proxyGet<PublicKeyResponse>(
  "/api/notifications/push/public-key",
);
