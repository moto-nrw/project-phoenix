import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { NextRequest } from "next/server";
import { withPrivateNoStore } from "~/lib/private-no-store";

/** List public tenant names without attaching a school session. */
const proxy = createPublicJsonProxy({
  method: "GET",
  path: "/auth/tenants",
  contentTypeOnGet: false,
});

// Keep the no-argument entry point used by existing server-side callers.
export const GET = withPrivateNoStore(
  (
    request: NextRequest = new NextRequest("http://localhost/api/tenant/list"),
  ) => proxy(request),
);
