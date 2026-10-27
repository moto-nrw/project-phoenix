import { traegerHandlers } from "~/server/auth/traeger";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { withBrowserSession } from "~/server/auth/browser-session";

export const GET = withPrivateNoStore(withBrowserSession(traegerHandlers.GET));
export const POST = withPrivateNoStore(
  withBrowserSession(traegerHandlers.POST),
);
