import { createLogger } from "~/lib/logger";
import { createOptionalTenantJsonProxy } from "~/lib/route-wrapper.server";

const logger = createLogger({ component: "AuthRegisterRoute" });

// Public registration may carry an existing tenant session for admin role checks.
export const POST = createOptionalTenantJsonProxy({
  method: "POST",
  path: "/auth/register",
  onBackendError: (response) => {
    logger.error("registration failed", {
      status: response.status,
      content_type: response.headers.get("content-type"),
    });
  },
});
