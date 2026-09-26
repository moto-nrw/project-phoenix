import { NextResponse } from "next/server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

export const POST = createPublicJsonProxy({
  method: "POST",
  path: "/operator/auth/email-confirm",
  forwardClientHeaders: true,
  invalidJsonResponse: () =>
    NextResponse.json({ message: "Ungültige Anfrage" }, { status: 400 }),
  networkErrorResponse: () =>
    NextResponse.json(
      { message: "Ein interner Fehler ist aufgetreten" },
      { status: 500 },
    ),
});
