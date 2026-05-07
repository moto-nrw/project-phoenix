import { NextResponse } from "next/server";

// Stub route required by Turbopack to register the deeper /decide
// route. No per-child read endpoint is exposed admin-side — child
// data comes via the parent request aggregate.
export async function GET() {
  return NextResponse.json({ error: "Method Not Allowed" }, { status: 405 });
}
