import {
  createParentDeleteHandler,
  parentApiDelete,
  proxyGet,
} from "~/lib/parent/route-wrapper.server";

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
 * GET /api/parent/me/notification-preferences — which notifications this
 * guardian agreed to, merged across every school the family belongs to.
 */
export const GET = proxyGet<NotificationPreferencesResponse>(
  "/parent/me/notification-preferences",
);

/**
 * DELETE /api/parent/me/notification-preferences — switch every type off.
 */
export const DELETE = createParentDeleteHandler<null>(
  async (_request, token) => {
    await parentApiDelete<null>("/parent/me/notification-preferences", token);
    return null;
  },
);
