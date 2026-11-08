import type { NextRequest } from "next/server";
import { forwardDemoToken } from "../forward";
import { withPrivateNoStore } from "~/lib/private-no-store";

async function POSTHandler(request: NextRequest) {
  return forwardDemoToken(request, "status");
}

export const POST = withPrivateNoStore(POSTHandler);
