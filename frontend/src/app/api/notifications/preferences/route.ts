import { apiDelete } from "~/lib/api-helpers.server";
import { proxyGet } from "~/lib/route-proxy.server";
import { createDeleteHandler } from "~/lib/route-wrapper.server";

interface NotificationPreferencesResponse {
  tenant_enabled: boolean;
  types: {
    key: string;
    label: string;
    description: string;
    group: string;
    enabled: boolean;
    available: boolean;
  }[];
}

/**
 * GET /api/notifications/preferences — which notifications the logged-in user
 * agreed to, plus the school-wide state of each type.
 */
export const GET = proxyGet<NotificationPreferencesResponse>(
  "/api/notifications/preferences/",
);

/**
 * DELETE /api/notifications/preferences — switch every type off at once
 * ("Alle deaktivieren").
 */
export const DELETE = createDeleteHandler<null>(async (_request, token) => {
  await apiDelete("/api/notifications/preferences/", token);
  return null;
});
