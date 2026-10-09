import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { operatorErrorResponse } from "~/lib/operator/route-wrapper.server";

export const POST = createPublicJsonProxy({
  method: "POST",
  path: "/operator/auth/email-confirm",
  forwardClientHeaders: true,
  invalidJsonResponse: () =>
    operatorErrorResponse(400, "general.input", "Invalid JSON request body"),
  networkErrorResponse: () =>
    operatorErrorResponse(503, "general.unavailable", "Backend request failed"),
});
