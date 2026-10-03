import { NextResponse } from "next/server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

export const POST = createPublicJsonProxy({
  method: "POST",
  path: "/operator/auth/email-confirm",
  forwardClientHeaders: true,
  invalidJsonResponse: () =>
    NextResponse.json(
      { status: "error", error: "Ungültige Anfrage" },
      { status: 400 },
    ),
  networkErrorResponse: () =>
    NextResponse.json(
      { status: "error", error: "Ein interner Fehler ist aufgetreten" },
      { status: 500 },
    ),
});
