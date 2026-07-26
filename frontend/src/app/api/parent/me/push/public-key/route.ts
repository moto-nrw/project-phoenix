import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface PublicKeyResponse {
  public_key: string;
}

/**
 * GET /api/parent/me/push/public-key — VAPID public key for Web Push
 * subscription in the parents portal (#2003).
 */
export const GET = proxyGet<PublicKeyResponse>("/parent/me/push/public-key");
