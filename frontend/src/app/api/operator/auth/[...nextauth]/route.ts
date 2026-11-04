import { operatorHandlers } from "~/server/auth/operator";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { withBrowserSession } from "~/server/auth/browser-session";

export const GET = withPrivateNoStore(withBrowserSession(operatorHandlers.GET));
export const POST = withPrivateNoStore(
  withBrowserSession(operatorHandlers.POST),
);
