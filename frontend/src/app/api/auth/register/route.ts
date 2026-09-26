import { NextResponse } from "next/server";
import { createLogger } from "~/lib/logger";
import { createOptionalTenantJsonProxy } from "~/lib/backend-proxy-route.server";

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
  networkErrorResponse: (error) =>
    NextResponse.json(
      {
        message: "An error occurred during registration",
        error: String(error),
      },
      { status: 500 },
    ),
});
