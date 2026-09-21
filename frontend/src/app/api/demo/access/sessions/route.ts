import type { NextRequest } from "next/server";
import { demoBackendUrl, forwardDemoToken } from "../forward";

export async function POST(request: NextRequest) {
  return forwardDemoToken(request, (token, headers) =>
    fetch(demoBackendUrl("sessions"), {
      method: "POST",
      headers: { ...headers, "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
    }),
  );
}
