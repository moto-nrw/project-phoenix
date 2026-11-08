import { catererPortal } from "~/server/auth/caterer";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { withBrowserSession } from "~/server/auth/browser-session";

export const GET = withPrivateNoStore(
  withBrowserSession(catererPortal.handlers.GET),
);
export const POST = withPrivateNoStore(
  withBrowserSession(catererPortal.handlers.POST),
);
