import { tenantPortal } from "~/server/auth";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { withBrowserSession } from "~/server/auth/browser-session";

export const GET = withPrivateNoStore(
  withBrowserSession(tenantPortal.handlers.GET),
);
export const POST = withPrivateNoStore(
  withBrowserSession(tenantPortal.handlers.POST),
);
