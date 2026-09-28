import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

// Admin permission and preview eligibility are enforced by the backend.
// Forward the real browser identity because minting writes an audit event.
export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/auth/staff-preview",
  forwardClientHeaders: true,
});
