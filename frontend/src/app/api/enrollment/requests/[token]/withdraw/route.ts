import { NextResponse } from "next/server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const token = params.token;
  if (typeof token !== "string" || !token) {
    return NextResponse.json({ error: "token required" }, { status: 400 });
  }
  return `/api/enrollment/requests/${encodeURIComponent(token)}/withdraw`;
};

export const POST = createPublicJsonProxy({
  method: "POST",
  path,
  body: "optional-json",
});
