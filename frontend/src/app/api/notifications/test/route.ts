import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

/**
 * POST /api/notifications/test — proxy to the backend test-notification
 * trigger (#1624). Lets a staff user verify their notification setup end to end:
 * feature flag on → backend dispatch → SSE → in-app toast.
 */
export const POST = createPostHandler<null, Record<string, never>>(
  async (_request, _body, token: string) => {
    await apiPost(`/api/notifications/test`, token);
    return null;
  },
);
