import { NextResponse } from "next/server";
import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

/** Bounds the path segment before it reaches the backend. */
const VALID_TYPE_PATTERN = /^[a-z0-9_]{1,64}$/;

function emailSubscriptionPath(
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) {
  const type = params.type;
  if (typeof type !== "string" || !VALID_TYPE_PATTERN.test(type)) {
    return NextResponse.json(
      { error: "Invalid notification type format" },
      { status: 400 },
    );
  }
  return `/api/notifications/email-subscriptions/${type}`;
}

/** Eigene Entscheidung für eine E-Mail-Benachrichtigung lesen (#3780). */
export const GET = createTenantJsonProxy({
  method: "GET",
  path: emailSubscriptionPath,
  cache: "no-store",
});

/** Eigene Entscheidung für eine E-Mail-Benachrichtigung setzen (#3780). */
export const PUT = createTenantJsonProxy({
  method: "PUT",
  path: emailSubscriptionPath,
});
