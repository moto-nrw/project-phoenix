import { parentHandlers } from "~/server/auth/parent";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { withBrowserSession } from "~/server/auth/browser-session";

export const GET = withPrivateNoStore(withBrowserSession(parentHandlers.GET));
export const POST = withPrivateNoStore(withBrowserSession(parentHandlers.POST));
