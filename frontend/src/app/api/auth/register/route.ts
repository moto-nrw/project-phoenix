import { createLogger } from "~/lib/logger";
import { createOptionalTenantJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "AuthRegisterRoute" });

// Public registration may carry an existing tenant session for admin role checks.
export const POST = createOptionalTenantJsonProxy({
  method: "POST",
  path: "/auth/register",
  forwardClientHeaders: true,
  onBackendError: (response) => {
    logger.error("registration failed", {
      status: response.status,
      content_type: response.headers.get("content-type"),
    });
  },
  networkErrorResponse: (error, request) =>
    problemResponse(
      500,
      `registration request failed: ${String(error)}`,
      request,
    ),
});
