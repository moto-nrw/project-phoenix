import { operatorPortal } from "~/server/auth/operator";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { withBrowserSession } from "~/server/auth/browser-session";

export const GET = withPrivateNoStore(
  withBrowserSession(operatorPortal.handlers.GET),
);
export const POST = withPrivateNoStore(
  withBrowserSession(operatorPortal.handlers.POST),
);
