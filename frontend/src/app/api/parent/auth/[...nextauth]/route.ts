import { parentPortal } from "~/server/auth/parent";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { withBrowserSession } from "~/server/auth/browser-session";

export const GET = withPrivateNoStore(
  withBrowserSession(parentPortal.handlers.GET),
);
export const POST = withPrivateNoStore(
  withBrowserSession(parentPortal.handlers.POST),
);
