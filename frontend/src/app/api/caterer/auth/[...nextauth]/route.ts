import { catererHandlers } from "~/server/auth/caterer";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { withBrowserSession } from "~/server/auth/browser-session";

export const GET = withPrivateNoStore(withBrowserSession(catererHandlers.GET));
export const POST = withPrivateNoStore(
  withBrowserSession(catererHandlers.POST),
);
