import { schoolHandlers } from "~/server/auth/school";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { withBrowserSession } from "~/server/auth/browser-session";

export const GET = withPrivateNoStore(withBrowserSession(schoolHandlers.GET));
export const POST = withPrivateNoStore(withBrowserSession(schoolHandlers.POST));
