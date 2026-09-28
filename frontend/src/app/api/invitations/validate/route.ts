import { NextResponse } from "next/server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createPublicJsonProxy({
  method: "GET",
  path: (request) => {
    const token = request.nextUrl.searchParams.get("token");
    if (!token) {
      return NextResponse.json(
        { error: "Missing invitation token" },
        { status: 400 },
      );
    }
    return `/auth/invitations/${encodeURIComponent(token)}`;
  },
  contentTypeOnGet: false,
});
