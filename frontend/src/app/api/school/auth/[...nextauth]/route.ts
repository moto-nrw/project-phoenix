import { schoolPortal } from "~/server/auth/school";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { withBrowserSession } from "~/server/auth/browser-session";

export const GET = withPrivateNoStore(
  withBrowserSession(schoolPortal.handlers.GET),
);
export const POST = withPrivateNoStore(
  withBrowserSession(schoolPortal.handlers.POST),
);
