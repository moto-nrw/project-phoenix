import type { NextRequest } from "next/server";
import { demoBackendUrl, forwardDemoToken } from "../forward";
import { withPrivateNoStore } from "~/lib/private-no-store";

async function POSTHandler(request: NextRequest) {
  return forwardDemoToken(request, (token, headers) =>
    fetch(demoBackendUrl("status"), {
      headers: { ...headers, Authorization: `Bearer ${token}` },
    }),
  );
}

export const POST = withPrivateNoStore(POSTHandler);
