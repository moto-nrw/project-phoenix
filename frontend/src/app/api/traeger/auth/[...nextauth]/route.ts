import { traegerPortal } from "~/server/auth/traeger";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { withBrowserSession } from "~/server/auth/browser-session";

export const GET = withPrivateNoStore(
  withBrowserSession(traegerPortal.handlers.GET),
);
export const POST = withPrivateNoStore(
  withBrowserSession(traegerPortal.handlers.POST),
);
