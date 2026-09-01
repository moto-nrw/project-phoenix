import { proxyPost } from "~/lib/school/route-wrapper.server";

// Testbenachrichtigung an das eigene Konto (#2208).
export const POST = proxyPost<null, Record<string, never>>(
  "/school/notifications/test",
);
