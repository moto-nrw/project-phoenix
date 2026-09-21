import type { NextRequest } from "next/server";
import { demoBackendUrl, forwardDemoToken } from "../forward";

export async function POST(request: NextRequest) {
  return forwardDemoToken(request, (token, headers) =>
    fetch(demoBackendUrl("status"), {
      headers: { ...headers, Authorization: `Bearer ${token}` },
    }),
  );
}
