import { createLogger } from "~/lib/logger";
import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

const logger = createLogger({ component: "AuthLinkToTenantRoute" });

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/auth/link-to-tenant",
  unauthorizedError: "Nicht authentifiziert",
  onBackendError: (response) => {
    logger.error("link_to_tenant_failed", { status: response.status });
  },
});
