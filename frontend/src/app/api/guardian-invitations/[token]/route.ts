import { NextResponse } from "next/server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createPublicJsonProxy({
  method: "GET",
  path: (_request, params) => {
    const token = params.token;
    if (typeof token !== "string" || !token) {
      return NextResponse.json(
        { error: "Missing invitation token" },
        { status: 400 },
      );
    }
    return `/auth/guardian-invitations/${encodeURIComponent(token)}`;
  },
  contentTypeOnGet: false,
});
